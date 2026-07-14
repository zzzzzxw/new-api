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
		body, size, closer, err := relaycommon.NewOutboundJSONBody(jsonData)
		if err != nil {
			return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		defer closer.Close()
		jsonData = nil
		info.UpstreamRequestBodySize = size
		requestBody = body
		info.CopyRequestParametersToUpstream()
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

// normalizeResponsesInputIDsJSON rewrites input item IDs in a raw Responses
// API JSON body so that every `id` starts with "msg_". This is used for the
// passthrough path where the request body bypasses the structured DTO.
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
		id, ok := item["id"].(string)
		if !ok || id == "" || strings.HasPrefix(id, "msg_") {
			continue
		}
		item["id"] = "msg_" + id
		input[i] = item
		changed = true
	}
	if !changed {
		return data, nil
	}
	body["input"] = input
	return common.Marshal(body)
}

// normalizeResponsesInputIDs rewrites any input item `id` field that does not
// start with "msg_" to have that prefix. The Responses API validates that all
// IDs begin with "msg_". This handles legacy IDs from chat→responses conversion
// that used a "chatcmpl-..._msg_0" format, so users can switch between chat and
// responses models mid-conversation without hitting ID validation errors.
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
		id, ok := items[i]["id"].(string)
		if !ok || id == "" || strings.HasPrefix(id, "msg_") {
			continue
		}
		items[i]["id"] = "msg_" + id
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
