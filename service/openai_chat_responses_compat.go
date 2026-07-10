package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service/relayconvert"
)

func ChatCompletionsRequestToResponsesRequest(req *dto.GeneralOpenAIRequest) (*dto.OpenAIResponsesRequest, error) {
	return relayconvert.ChatCompletionsRequestToResponsesRequest(req)
}

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	return relayconvert.ResponsesRequestToChatCompletionsRequest(req)
}

func NormalizeCodexChatReasoningEffort(effort string, mode string) string {
	return relayconvert.NormalizeCodexChatReasoningEffort(effort, mode)
}

func ApplyChannelReasoningEffortMapping(info *relaycommon.RelayInfo, model string, originalEffort string, defaultEffort string) string {
	if info == nil || info.ChannelMeta == nil {
		return defaultEffort
	}
	if info.ReasoningEffortEnabled != nil && !*info.ReasoningEffortEnabled {
		info.ReasoningEffortResolved = true
		return ""
	}

	rawMapping := strings.TrimSpace(info.ReasoningEffortMapping)
	if rawMapping == "" {
		return defaultEffort
	}
	var rules []dto.ReasoningEffortMappingRule
	if err := common.UnmarshalJsonStr(rawMapping, &rules); err != nil {
		return defaultEffort
	}

	upstreamModel := strings.TrimSpace(info.UpstreamModelName)
	if upstreamModel == "" {
		upstreamModel = strings.TrimSpace(model)
	}
	normalizedOriginal := strings.ToLower(strings.TrimSpace(originalEffort))
	for _, rule := range rules {
		if strings.TrimSpace(rule.Model) != upstreamModel {
			continue
		}
		if strings.ToLower(strings.TrimSpace(rule.OriginalReasoningEffort)) != normalizedOriginal {
			continue
		}
		info.ReasoningEffortResolved = true
		replacement := strings.ToLower(strings.TrimSpace(rule.ReplacementReasoningEffort))
		if replacement == "none" || replacement == "off" || replacement == "disabled" {
			return ""
		}
		return replacement
	}
	return defaultEffort
}

func SanitizeChatCompletionsToolsJSON(data []byte) ([]byte, bool, error) {
	return relayconvert.SanitizeChatCompletionsToolsJSON(data)
}

func SanitizeResponsesToolsJSON(data []byte) ([]byte, bool, error) {
	return relayconvert.SanitizeResponsesToolsJSON(data)
}

func ChatCompletionsResponseToResponsesResponse(resp *dto.OpenAITextResponse, id string) (*dto.OpenAIResponsesResponse, *dto.Usage, error) {
	return relayconvert.ChatCompletionsResponseToResponsesResponse(resp, id)
}

func ResponsesResponseToChatCompletionsResponse(resp *dto.OpenAIResponsesResponse, id string) (*dto.OpenAITextResponse, *dto.Usage, error) {
	return relayconvert.ResponsesResponseToChatCompletionsResponse(resp, id)
}

func ResponsesFinishReasonFromStatus(resp *dto.OpenAIResponsesResponse) (string, bool) {
	return relayconvert.ResponsesFinishReasonFromStatus(resp)
}

func ExtractOutputTextFromResponses(resp *dto.OpenAIResponsesResponse) string {
	return relayconvert.ExtractOutputTextFromResponses(resp)
}
