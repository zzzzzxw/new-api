package openai

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/relayconvert"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func OaiChatToResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var chatResp dto.OpenAITextResponse
	if err := common.Unmarshal(body, &chatResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := chatResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		logger.LogWarn(c, fmt.Sprintf("[CHAT2RESP-ERROR] status=%d type=%s code=%v", resp.StatusCode, oaiError.Type, oaiError.Code))
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	responseID := helper.GetResponseID(c)
	responsesResp, usage, err := service.ChatCompletionsResponseToResponsesResponse(&chatResp, responseID)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if usage == nil || usage.TotalTokens == 0 {
		text := service.ExtractOutputTextFromResponses(responsesResp)
		usage = service.ResponseText2Usage(c, text, info.UpstreamModelName, info.GetEstimatePromptTokens())
		responsesResp.Usage = relayconvert.UsageFromChatUsage(usage)
	}

	responseBody, err := common.Marshal(responsesResp)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

// isResponsesMilestoneEvent returns true for Responses SSE events that represent
// significant milestones rather than incremental deltas or progress updates.
func isResponsesMilestoneEvent(eventType string) bool {
	switch eventType {
	case
		"response.output_item.added",
		"response.reasoning_summary_part.added",
		"response.reasoning_summary_part.done",
		"response.function_call_arguments.done",
		"response.custom_tool_call_input.done",
		"response.completed",
		"response.incomplete",
		"response.failed":
		return true
	}
	return false
}

func OaiChatToResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	responseID := helper.GetResponseID(c)
	state := relayconvert.NewChatToResponsesStreamState(responseID, info.UpstreamModelName)
	streamErr := (*types.NewAPIError)(nil)

	sendEvents := func(events []relayconvert.ChatToResponsesStreamEvent) bool {
		if len(events) == 0 {
			return true
		}
		for _, event := range events {
			data, err := common.Marshal(event.Payload)
			if err != nil {
				streamErr = types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
				return false
			}
			if err := helper.ResponseChunkDataNoFlush(c, dto.ResponsesStreamResponse{Type: event.Type}, string(data)); err != nil {
				streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
				return false
			}
		}
		if err := helper.FlushWriter(c); err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			return false
		}
		return true
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		if streamErr != nil {
			sr.Stop(streamErr)
			return
		}

		if strings.Contains(data, `"error"`) {
			var errorResp dto.OpenAITextResponse
			if err := common.UnmarshalJsonStr(data, &errorResp); err == nil {
				if oaiError := errorResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
					logger.LogWarn(c, fmt.Sprintf("[CHAT2RESP-ERROR] status=%d type=%s code=%v", resp.StatusCode, oaiError.Type, oaiError.Code))
					streamErr = types.WithOpenAIError(*oaiError, resp.StatusCode)
					sr.Stop(streamErr)
					return
				}
			}
		}

		var chunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &chunk); err != nil {
			logger.LogError(c, "failed to unmarshal chat stream response: "+err.Error())
			sr.Error(err)
			return
		}

		events, err := relayconvert.ChatCompletionsStreamChunkToResponsesEvents(&chunk, state)
		if err != nil {
			streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError)
			sr.Stop(streamErr)
			return
		}
		if !sendEvents(events) {
			sr.Stop(streamErr)
			return
		}
	})

	if streamErr != nil {
		return nil, streamErr
	}

	usage := state.Usage
	if usage == nil || usage.TotalTokens == 0 {
		usage = service.ResponseText2Usage(c, state.UsageText(), info.UpstreamModelName, info.GetEstimatePromptTokens())
		state.Usage = relayconvert.UsageFromChatUsage(usage)
	}

	if !sendEvents(relayconvert.FinalizeChatCompletionsStreamToResponses(state)) {
		return nil, streamErr
	}

	return usage, nil
}
