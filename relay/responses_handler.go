package relay

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	appconstant "github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func ResponsesHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		switch info.ApiType {
		case appconstant.APITypeOpenAI, appconstant.APITypeCodex:
		default:
			return types.NewErrorWithStatusCode(
				fmt.Errorf("unsupported endpoint %q for api type %d", "/v1/responses/compact", info.ApiType),
				types.ErrorCodeInvalidRequest,
				http.StatusBadRequest,
				types.ErrOptionWithSkipRetry(),
			)
		}
	}

	var responsesReq *dto.OpenAIResponsesRequest
	switch req := info.Request.(type) {
	case *dto.OpenAIResponsesRequest:
		responsesReq = req
	case *dto.OpenAIResponsesCompactionRequest:
		responsesReq = &dto.OpenAIResponsesRequest{
			Model:              req.Model,
			Input:              req.Input,
			Instructions:       req.Instructions,
			PreviousResponseID: req.PreviousResponseID,
		}
	default:
		return types.NewErrorWithStatusCode(
			fmt.Errorf("invalid request type, expected dto.OpenAIResponsesRequest or dto.OpenAIResponsesCompactionRequest, got %T", info.Request),
			types.ErrorCodeInvalidRequest,
			http.StatusBadRequest,
			types.ErrOptionWithSkipRetry(),
		)
	}

	request, err := common.DeepCopy(responsesReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to GeneralOpenAIRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	info.SetRequestParametersFromValue(responsesReq)

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	if err := normalizeResponsesInputIDs(request); err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)
	var requestBody io.Reader
	if model_setting.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := common.GetBodyStorage(c)
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		jsonData, err := storage.Bytes()
		if err != nil {
			return types.NewError(err, types.ErrorCodeReadRequestBodyFailed, types.ErrOptionWithSkipRetry())
		}
		jsonData, err = normalizeResponsesInputIDsJSON(jsonData)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		jsonData, err = forceCodexResponsesStream(jsonData, info)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		if info.ApiType == appconstant.APITypeCodex && info.RelayMode == relayconstant.RelayModeResponses {
			info.SetUpstreamRequestParametersFromJSON(jsonData)
		}
		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		jsonData = nil
		info.UpstreamRequestBodySize = size
		requestBody = body
		if info.ApiType != appconstant.APITypeCodex || info.RelayMode != relayconstant.RelayModeResponses {
			info.CopyRequestParametersToUpstream()
		}
	} else {
		convertedRequest, err := adaptor.ConvertOpenAIResponsesRequest(c, info, *request)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		relaycommon.AppendRequestConversionFromRequest(info, convertedRequest)
		jsonData, err := common.Marshal(convertedRequest)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// remove disabled fields for OpenAI Responses API
		jsonData, err = relaycommon.RemoveDisabledFields(jsonData, info.ChannelOtherSettings, info.ChannelSetting.PassThroughBodyEnabled)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		// apply param override
		if len(info.ParamOverride) > 0 {
			jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
			if err != nil {
				return newAPIErrorFromParamOverride(err)
			}
		}
		jsonData, err = sanitizeConvertedResponsesChatRequestJSON(convertedRequest, jsonData)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		jsonData, err = forceCodexResponsesStream(jsonData, info)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}

		logger.LogDebug(c, "requestBody: %s", jsonData)
		info.SetUpstreamRequestParametersFromJSON(jsonData)
		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		jsonData = nil
		info.UpstreamRequestBodySize = size
		requestBody = body
	}

	var httpResp *http.Response
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")

	if resp != nil {
		httpResp = resp.(*http.Response)

		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}

	usageDto := usage.(*dto.Usage)
	if info.RelayMode == relayconstant.RelayModeResponsesCompact {
		originModelName := info.OriginModelName
		originPriceData := info.PriceData

		_, err := helper.ModelPriceHelper(c, info, info.GetEstimatePromptTokens(), &types.TokenCountMeta{})
		if err != nil {
			info.OriginModelName = originModelName
			info.PriceData = originPriceData
			return types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithSkipRetry(), types.ErrOptionWithStatusCode(http.StatusBadRequest))
		}
		service.PostTextConsumeQuota(c, info, usageDto, nil)

		info.OriginModelName = originModelName
		info.PriceData = originPriceData
		return nil
	}

	if strings.HasPrefix(info.OriginModelName, "gpt-4o-audio") {
		service.PostAudioConsumeQuota(c, info, usageDto, "")
	} else {
		service.PostTextConsumeQuota(c, info, usageDto, nil)
	}
	return nil
}

func sanitizeConvertedResponsesChatRequestJSON(convertedRequest any, jsonData []byte) ([]byte, error) {
	if _, ok := convertedRequest.(*dto.GeneralOpenAIRequest); !ok {
		if shouldSanitizeResponsesToolsForUpstream(convertedRequest, jsonData) {
			sanitized, _, err := service.SanitizeResponsesToolsJSON(jsonData)
			if err != nil {
				return nil, err
			}
			return sanitized, nil
		}
		return jsonData, nil
	}
	sanitized, _, err := service.SanitizeChatCompletionsToolsJSON(jsonData)
	if err != nil {
		return nil, err
	}
	return sanitized, nil
}

func shouldSanitizeResponsesToolsForUpstream(convertedRequest any, jsonData []byte) bool {
	switch convertedRequest.(type) {
	case dto.OpenAIResponsesRequest, *dto.OpenAIResponsesRequest:
	default:
		return false
	}
	model := strings.ToLower(strings.TrimSpace(gjson.GetBytes(jsonData, "model").String()))
	return strings.Contains(model, "glm-")
}

// responsesExpectedIDPrefix returns the ID prefix the Responses API expects
// for a given input item type. Message items use "msg_", reasoning uses "rs_",
// function calls use "fc_", and so on. Returns "" when the type is unknown or
// has no fixed prefix requirement.
func responsesExpectedIDPrefix(itemType string) string {
	switch strings.TrimSpace(itemType) {
	case "message":
		return "msg_"
	case "reasoning":
		return "rs_"
	case "function_call":
		return "fc_"
	case "custom_tool_call":
		return "ctc_"
	case "tool_search_call":
		return "tsc_"
	case "file_search_call":
		return "fs_"
	case "web_search_call":
		return "ws_"
	case "computer_call":
		return "cu_"
	case "image_generation_call":
		return "img_"
	case "code_interpreter_call":
		return "ci_"
	default:
		return ""
	}
}

// normalizeInputItemID checks whether an input item's id already has the prefix
// expected for its type. If the id has a recognized wrong prefix (chatcmpl-,
// call_, etc. from the chat completions world) or is missing the expected one,
// it prepends the correct prefix. IDs that already match are left untouched.
func normalizeInputItemID(item map[string]any) (string, bool) {
	id, ok := item["id"].(string)
	if !ok || id == "" {
		return "", false
	}
	itemType, _ := item["type"].(string)
	expected := responsesExpectedIDPrefix(itemType)
	if expected == "" || strings.HasPrefix(id, expected) {
		return id, false
	}
	if strings.HasPrefix(id, "chatcmpl-") ||
		strings.HasPrefix(id, "call_") ||
		strings.HasPrefix(id, "msg_") ||
		strings.HasPrefix(id, "rs_") ||
		strings.HasPrefix(id, "fc_") {
		return expected + id, true
	}
	return expected + id, true
}

// normalizeResponsesInputIDsJSON fixes input item IDs in a raw Responses API
// JSON body for the passthrough path. Each input item type has an expected ID
// prefix (msg_ for messages, rs_ for reasoning, fc_ for function calls, etc.).
// IDs from the chat completions world (chatcmpl-, call_) are rewritten to use
// the correct prefix.
func normalizeResponsesInputIDsJSON(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}
	var body map[string]any
	if err := common.Unmarshal(data, &body); err != nil {
		return data, nil
	}
	input, ok := body["input"].([]any)
	if !ok || len(input) == 0 {
		return data, nil
	}
	changed := false
	for i := range input {
		item, ok := input[i].(map[string]any)
		if !ok {
			continue
		}
		newID, itemChanged := normalizeInputItemID(item)
		if !itemChanged {
			continue
		}
		item["id"] = newID
		input[i] = item
		changed = true
	}
	if !changed {
		return data, nil
	}
	body["input"] = input
	return common.Marshal(body)
}

// normalizeResponsesInputIDs fixes input item IDs that were generated by the
// chat→responses conversion path. Each item type expects a specific ID prefix
// (msg_ for messages, rs_ for reasoning, fc_ for function calls, etc.). IDs
// from the chat completions world (chatcmpl-, call_) are rewritten to use the
// correct prefix.
func normalizeResponsesInputIDs(request *dto.OpenAIResponsesRequest) error {
	if request == nil || len(request.Input) == 0 || common.GetJsonType(request.Input) != "array" {
		return nil
	}

	var items []map[string]any
	if err := common.Unmarshal(request.Input, &items); err != nil {
		return err
	}

	changed := false
	for i := range items {
		newID, itemChanged := normalizeInputItemID(items[i])
		if !itemChanged {
			continue
		}
		items[i]["id"] = newID
		changed = true
	}

	if !changed {
		return nil
	}

	fixed, err := common.Marshal(items)
	if err != nil {
		return err
	}
	request.Input = fixed

	return nil
}
