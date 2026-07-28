package groksubscription

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/xai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	xai.Adaptor
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil || info.RelayMode != relayconstant.RelayModeResponses {
		return "", errors.New("grok subscription channel: only the Responses API is supported upstream")
	}
	return relaycommon.GetFullRequestURL(info.ChannelBaseUrl, "/v1/responses", info.ChannelType), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	oauthKey, err := ParseOAuthKey(strings.TrimSpace(info.ApiKey))
	if err != nil {
		return err
	}
	if strings.TrimSpace(oauthKey.AccessToken) == "" {
		return errors.New("grok subscription channel: access_token is required")
	}

	req.Set("Authorization", "Bearer "+strings.TrimSpace(oauthKey.AccessToken))
	req.Set("Content-Type", "application/json")
	req.Set("Accept", "application/json, text/event-stream")
	req.Set("User-Agent", GrokUserAgent)
	req.Set("X-Grok-Client-Version", GrokCLIVersion())
	req.Set("X-Grok-Client-Mode", "interactive")
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("grok subscription channel: Chat Completions must be converted to Responses")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	converted, err := a.Adaptor.ConvertOpenAIResponsesRequest(c, info, request)
	if err != nil {
		return nil, err
	}
	normalized, ok := converted.(dto.OpenAIResponsesRequest)
	if !ok {
		return converted, nil
	}

	normalized.PromptCacheRetention = nil
	normalized.SafetyIdentifier = nil
	if strings.Contains(strings.ToLower(normalized.Model), "composer") {
		normalized.Reasoning = nil
	}
	if err := normalizeGrokInputAndTools(c, &normalized); err != nil {
		return nil, err
	}
	return normalized, nil
}

func normalizeGrokInputAndTools(c *gin.Context, request *dto.OpenAIResponsesRequest) error {
	if request == nil {
		return nil
	}

	var input any
	if len(request.Input) > 0 {
		_ = common.Unmarshal(request.Input, &input)
	}

	var tools []map[string]any
	if len(request.Tools) > 0 {
		_ = common.Unmarshal(request.Tools, &tools)
	}

	if inputItems, ok := input.([]any); ok {
		filteredInput := make([]any, 0, len(inputItems))
		for _, rawItem := range inputItems {
			if item, ok := rawItem.(map[string]any); ok {
				deleteFieldsRecursive(item, "external_web_access")
				if itemType, _ := item["type"].(string); itemType == "additional_tools" {
					if additional, ok := item["tools"].([]any); ok {
						for _, rawTool := range additional {
							if tool, ok := rawTool.(map[string]any); ok {
								tools = append(tools, tool)
							}
						}
					}
					continue
				}
				if itemType, _ := item["type"].(string); itemType == "reasoning" && item["content"] == nil {
					delete(item, "content")
				}
			}
			filteredInput = append(filteredInput, rawItem)
		}
		encoded, err := common.Marshal(filteredInput)
		if err != nil {
			return err
		}
		request.Input = encoded
		input = filteredInput
	}

	supported := map[string]bool{
		"code_execution": true, "code_interpreter": true, "collections_search": true,
		"file_search": true, "function": true, "mcp": true, "shell": true,
		"web_search": true, "x_search": true,
	}
	customTools := make(map[string]bool)
	functionTools := make(map[string]bool)
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		switch toolType, _ := tool["type"].(string); toolType {
		case "custom":
			if strings.TrimSpace(name) != "" {
				customTools[strings.TrimSpace(name)] = true
			}
		case "function":
			if strings.TrimSpace(name) != "" {
				functionTools[strings.TrimSpace(name)] = true
			}
		}
	}
	for name := range customTools {
		if functionTools[name] {
			return errors.New("grok subscription channel: custom tool conflicts with function tool: " + name)
		}
	}

	filteredTools := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		deleteFieldsRecursive(tool, "external_web_access")
		toolType, _ := tool["type"].(string)
		if toolType == "custom" {
			name, _ := tool["name"].(string)
			if customTools[strings.TrimSpace(name)] {
				tool["type"] = "function"
				tool["parameters"] = customToolInputSchema()
				delete(tool, "format")
				toolType = "function"
			}
		}
		if supported[toolType] {
			filteredTools = append(filteredTools, tool)
		}
	}
	if len(tools) > 0 {
		if len(filteredTools) == 0 {
			request.Tools = nil
			request.ToolChoice = nil
		} else {
			encoded, err := common.Marshal(filteredTools)
			if err != nil {
				return err
			}
			request.Tools = encoded
		}
	}
	if len(request.ToolChoice) > 0 {
		var choice map[string]any
		if err := common.Unmarshal(request.ToolChoice, &choice); err == nil {
			choiceType, _ := choice["type"].(string)
			choiceName, _ := choice["name"].(string)
			if choiceType == "custom" && customTools[strings.TrimSpace(choiceName)] {
				choice["type"] = "function"
				encoded, err := common.Marshal(choice)
				if err != nil {
					return err
				}
				request.ToolChoice = encoded
				choiceType = "function"
			}
			if choiceType != "" && !supported[choiceType] {
				request.ToolChoice = nil
			}
		}
	}
	if len(customTools) > 0 {
		if input != nil {
			rewriteCustomToolHistory(input, customTools)
			if encoded, err := common.Marshal(input); err != nil {
				return err
			} else {
				request.Input = encoded
			}
		}
		if c != nil {
			c.Set(grokCustomToolsContextKey, customTools)
		}
	}
	return nil
}

func deleteFieldsRecursive(value any, field string) {
	switch typed := value.(type) {
	case map[string]any:
		delete(typed, field)
		for _, child := range typed {
			deleteFieldsRecursive(child, field)
		}
	case []any:
		for _, child := range typed {
			deleteFieldsRecursive(child, field)
		}
	}
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if info != nil && info.RelayMode == relayconstant.RelayModeResponses {
		if err := prepareCustomToolResponse(c, resp, info.IsStream); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
	}
	return a.Adaptor.DoResponse(c, resp, info)
}
