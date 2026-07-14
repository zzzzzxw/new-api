package relayconvert

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/reasoning"
)

const (
	responsesInputTypeFunctionCall         = "function_call"
	responsesInputTypeFunctionCallOutput   = "function_call_output"
	responsesInputTypeCustomToolCall       = "custom_tool_call"
	responsesInputTypeCustomToolCallOutput = "custom_tool_call_output"
	responsesInputTypeToolSearchCall       = "tool_search_call"
	responsesInputTypeToolSearchOutput     = "tool_search_output"
	responsesInputTypeReasoning            = "reasoning"
	chatCompatWrappedToolPrefix            = "codex__"
	chatCompatWrappedToolOriginalTypeKey   = "x_newapi_responses_tool_type"
	chatCompatWrappedToolOriginalNameKey   = "x_newapi_responses_tool_name"
	chatCompatWrappedToolPayloadKey        = "input"
)

var chatCompatFunctionNameInvalidChars = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func ResponsesRequestToChatCompletionsRequest(req *dto.OpenAIResponsesRequest) (*dto.GeneralOpenAIRequest, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if req.Model == "" {
		return nil, errors.New("model is required")
	}
	if err := validateResponsesRequestChatUnsupportedFields(req); err != nil {
		return nil, err
	}

	messages, err := responsesRequestMessagesToChat(req)
	if err != nil {
		return nil, err
	}

	tools, err := responsesRequestToolsToChat(req.Tools)
	if err != nil {
		return nil, err
	}
	tools = appendChatToolsUnique(tools, responsesToolSearchOutputToolsToChat(req.Input)...)
	tools = normalizeChatCompletionTools(tools)

	toolChoice, err := responsesRequestToolChoiceToChat(req.ToolChoice)
	if err != nil {
		return nil, err
	}

	responseFormat, err := responsesRequestTextToChatResponseFormat(req.Text)
	if err != nil {
		return nil, err
	}

	model := req.Model
	if effort, originModel := reasoning.ParseOpenAIReasoningEffortFromModelSuffix(model); effort != "" {
		model = originModel
		if req.Reasoning == nil {
			req.Reasoning = &dto.Reasoning{Effort: effort}
		} else if req.Reasoning.Effort == "" {
			req.Reasoning.Effort = effort
		}
	}

	out := &dto.GeneralOpenAIRequest{
		Model:                model,
		Messages:             messages,
		Stream:               req.Stream,
		StreamOptions:        req.StreamOptions,
		MaxCompletionTokens:  req.MaxOutputTokens,
		Temperature:          req.Temperature,
		TopP:                 req.TopP,
		TopLogProbs:          req.TopLogProbs,
		ResponseFormat:       responseFormat,
		Tools:                tools,
		ToolChoice:           toolChoice,
		User:                 req.User,
		Store:                req.Store,
		Metadata:             req.Metadata,
		SafetyIdentifier:     req.SafetyIdentifier,
		PromptCacheRetention: req.PromptCacheRetention,
		EnableThinking:       req.EnableThinking,
	}

	if req.Reasoning != nil {
		out.ReasoningEffort = req.Reasoning.Effort
	}
	if req.ServiceTier != "" {
		out.ServiceTier, _ = common.Marshal(req.ServiceTier)
	}
	if len(req.ParallelToolCalls) > 0 && common.GetJsonType(req.ParallelToolCalls) == "boolean" {
		var parallelToolCalls bool
		if err := common.Unmarshal(req.ParallelToolCalls, &parallelToolCalls); err == nil {
			out.ParallelTooCalls = &parallelToolCalls
		}
	}
	if len(out.Tools) == 0 {
		out.ToolChoice = nil
		out.ParallelTooCalls = nil
	}
	if len(req.PromptCacheKey) > 0 && common.GetJsonType(req.PromptCacheKey) == "string" {
		var promptCacheKey string
		if err := common.Unmarshal(req.PromptCacheKey, &promptCacheKey); err == nil {
			out.PromptCacheKey = promptCacheKey
		}
	}

	return out, nil
}

// NormalizeCodexChatReasoningEffort mirrors Codex-chat proxy behavior: keep the
// official effort values when possible, but clamp values that common upstreams
// reject. Empty means the effort should be omitted.
func NormalizeCodexChatReasoningEffort(effort string, mode string) string {
	effort = strings.ToLower(strings.TrimSpace(effort))
	if effort == "" || effort == "none" || effort == "off" || effort == "disabled" {
		return ""
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "deepseek":
		if effort == "max" || effort == "xhigh" {
			return "max"
		}
		return "high"
	case "low_high":
		if effort == "minimal" || effort == "low" {
			return "low"
		}
		return "high"
	case "openrouter":
		switch effort {
		case "max", "xhigh":
			return "xhigh"
		case "high", "medium", "low", "minimal":
			return effort
		default:
			return ""
		}
	default:
		switch effort {
		case "minimal", "low", "medium", "high", "xhigh", "max":
			return effort
		default:
			return ""
		}
	}
}

func validateResponsesRequestChatUnsupportedFields(req *dto.OpenAIResponsesRequest) error {
	unsupported := make([]string, 0, 4)
	if rawJSONPresent(req.Conversation) {
		unsupported = append(unsupported, "conversation")
	}
	if strings.TrimSpace(req.PreviousResponseID) != "" {
		unsupported = append(unsupported, "previous_response_id")
	}
	if rawJSONPresent(req.Prompt) {
		unsupported = append(unsupported, "prompt")
	}
	if rawJSONPresent(req.ContextManagement) {
		unsupported = append(unsupported, "context_management")
	}
	if len(unsupported) > 0 {
		return fmt.Errorf("responses to chat conversion does not support stateful fields: %s", strings.Join(unsupported, ", "))
	}
	return nil
}

func responsesRequestMessagesToChat(req *dto.OpenAIResponsesRequest) ([]dto.Message, error) {
	messages := make([]dto.Message, 0)
	if rawJSONPresent(req.Instructions) {
		instructions, err := responsesJSONString(req.Instructions)
		if err != nil {
			return nil, fmt.Errorf("invalid instructions: %w", err)
		}
		if strings.TrimSpace(instructions) != "" {
			messages = append(messages, dto.Message{Role: "system", Content: instructions})
		}
	}

	if !rawJSONPresent(req.Input) {
		return messages, nil
	}

	switch common.GetJsonType(req.Input) {
	case "string":
		input, err := responsesJSONString(req.Input)
		if err != nil {
			return nil, fmt.Errorf("invalid input string: %w", err)
		}
		messages = append(messages, dto.Message{Role: "user", Content: input})
		return messages, nil
	case "array":
		var items []map[string]any
		if err := common.Unmarshal(req.Input, &items); err != nil {
			return nil, fmt.Errorf("invalid input array: %w", err)
		}
		var pendingReasoning string
		for _, item := range items {
			itemType := strings.TrimSpace(common.Interface2String(item["type"]))
			if itemType == responsesInputTypeReasoning {
				reasoning := strings.TrimSpace(responsesReasoningItemToContent(item))
				if reasoning != "" {
					if pendingReasoning != "" {
						pendingReasoning += "\n"
					}
					pendingReasoning += reasoning
				}
				continue
			}

			role := strings.TrimSpace(common.Interface2String(item["role"]))
			canHostReasoning := role == "assistant"
			switch itemType {
			case responsesInputTypeFunctionCall, responsesInputTypeCustomToolCall, responsesInputTypeToolSearchCall:
				canHostReasoning = true
			}
			if reasoning := strings.TrimSpace(pendingReasoning); reasoning != "" && !canHostReasoning {
				rc := reasoning
				messages = append(messages, dto.Message{
					Role:             "assistant",
					Content:          "",
					ReasoningContent: &rc,
				})
				pendingReasoning = ""
			}

			nextMessages, err := responsesInputItemToChatMessages(item, messages)
			if err != nil {
				return nil, err
			}
			messages = nextMessages
			// Attach accumulated reasoning to the assistant turn it precedes.
			// In Responses API, reasoning and message are separate items, but
			// Chat Completions collapses them into one assistant message with
			// both `reasoning_content` and `content`.
			if reasoning := strings.TrimSpace(pendingReasoning); reasoning != "" {
				if len(messages) > 0 && messages[len(messages)-1].Role == "assistant" {
					rc := reasoning
					messages[len(messages)-1].ReasoningContent = &rc
				} else {
					rc := reasoning
					messages = append(messages, dto.Message{
						Role:             "assistant",
						Content:          "",
						ReasoningContent: &rc,
					})
				}
				pendingReasoning = ""
			}
		}
		// Trailing reasoning with no following assistant turn still needs to be
		// surfaced so providers that require reasoning_content echo it back.
		if reasoning := strings.TrimSpace(pendingReasoning); reasoning != "" {
			if len(messages) > 0 && messages[len(messages)-1].Role == "assistant" {
				rc := reasoning
				messages[len(messages)-1].ReasoningContent = &rc
			} else {
				rc := reasoning
				messages = append(messages, dto.Message{
					Role:             "assistant",
					Content:          "",
					ReasoningContent: &rc,
				})
			}
		}
		return messages, nil
	default:
		return nil, fmt.Errorf("unsupported responses input type %q", common.GetJsonType(req.Input))
	}
}

func responsesInputItemToChatMessages(item map[string]any, messages []dto.Message) ([]dto.Message, error) {
	itemType := strings.TrimSpace(common.Interface2String(item["type"]))
	switch itemType {
	case responsesInputTypeFunctionCall:
		toolCall, err := responsesFunctionCallItemToChatToolCall(item)
		if err != nil {
			return nil, err
		}
		return appendToolCallToLastAssistant(messages, toolCall), nil
	case responsesInputTypeCustomToolCall:
		toolCall, err := responsesCustomToolCallItemToChatToolCall(item)
		if err != nil {
			return nil, err
		}
		return appendToolCallToLastAssistant(messages, toolCall), nil
	case responsesInputTypeToolSearchCall:
		return appendToolCallToLastAssistant(messages, responsesToolSearchCallItemToChatToolCall(item)), nil
	case responsesInputTypeFunctionCallOutput, responsesInputTypeToolSearchOutput, responsesInputTypeCustomToolCallOutput:
		callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
		content := responseToolOutputToChatContent(item["output"])
		return append(messages, dto.Message{Role: "tool", ToolCallId: callID, Content: content}), nil
	}

	role := strings.TrimSpace(common.Interface2String(item["role"]))
	if role == "" {
		role = "user"
	}
	content, err := responsesInputContentToChatContent(item["content"])
	if err != nil {
		return nil, err
	}
	return append(messages, dto.Message{Role: role, Content: content}), nil
}

func responsesInputContentToChatContent(content any) (any, error) {
	if content == nil {
		return "", nil
	}

	switch value := content.(type) {
	case string:
		return value, nil
	case []any:
		return responsesContentPartsToChatContent(value)
	case []map[string]any:
		parts := make([]any, 0, len(value))
		for _, part := range value {
			parts = append(parts, part)
		}
		return responsesContentPartsToChatContent(parts)
	default:
		return content, nil
	}
}

func responsesContentPartsToChatContent(parts []any) (any, error) {
	chatParts := make([]any, 0, len(parts))
	var textOnly strings.Builder
	onlyText := true

	for _, rawPart := range parts {
		part, ok := rawPart.(map[string]any)
		if !ok {
			onlyText = false
			chatParts = append(chatParts, rawPart)
			continue
		}

		partType := strings.TrimSpace(common.Interface2String(part["type"]))
		switch partType {
		case "input_text", "output_text", "text":
			text := common.Interface2String(part["text"])
			textOnly.WriteString(text)
			chatParts = append(chatParts, map[string]any{
				"type": dto.ContentTypeText,
				"text": text,
			})
		case "input_image":
			onlyText = false
			chatParts = append(chatParts, map[string]any{
				"type":      dto.ContentTypeImageURL,
				"image_url": responsesImagePartToChatImageURL(part),
			})
		case "input_file":
			onlyText = false
			chatParts = append(chatParts, map[string]any{
				"type": dto.ContentTypeFile,
				"file": responsesFilePartToChatFile(part),
			})
		case "input_audio":
			onlyText = false
			chatParts = append(chatParts, map[string]any{
				"type":        dto.ContentTypeInputAudio,
				"input_audio": responsesPartPayload(part, "input_audio"),
			})
		case "input_video":
			onlyText = false
			chatParts = append(chatParts, map[string]any{
				"type":      dto.ContentTypeVideoUrl,
				"video_url": responsesVideoPartToChatVideoURL(part),
			})
		default:
			onlyText = false
			chatParts = append(chatParts, part)
		}
	}

	if onlyText {
		return textOnly.String(), nil
	}
	return chatParts, nil
}

func responsesFunctionCallItemToChatToolCall(item map[string]any) (dto.ToolCallRequest, error) {
	name := strings.TrimSpace(common.Interface2String(item["name"]))
	if name == "" {
		return dto.ToolCallRequest{}, errors.New("function_call item is missing name")
	}
	return dto.ToolCallRequest{
		ID:   responsesCallID(item),
		Type: "function",
		Function: dto.FunctionRequest{
			Name:      name,
			Arguments: responsesArgumentsString(item["arguments"]),
		},
	}, nil
}

func responsesCustomToolCallItemToChatToolCall(item map[string]any) (dto.ToolCallRequest, error) {
	name := strings.TrimSpace(common.Interface2String(item["name"]))
	arguments := responsesArgumentsString(item["input"])
	if name == "" {
		name = responsesInputTypeCustomToolCall
	}
	return dto.ToolCallRequest{
		ID:   responsesCallID(item),
		Type: "function",
		Function: dto.FunctionRequest{
			Name:      chatCompatWrappedToolName(responsesInputTypeCustomToolCall, name),
			Arguments: arguments,
		},
	}, nil
}

func responsesToolSearchCallItemToChatToolCall(item map[string]any) dto.ToolCallRequest {
	arguments := responsesArgumentsString(item["arguments"])
	if strings.TrimSpace(arguments) == "" {
		arguments = responsesArgumentsString(item["input"])
	}
	return dto.ToolCallRequest{
		ID:   responsesCallID(item),
		Type: "function",
		Function: dto.FunctionRequest{
			Name:      "tool_search",
			Arguments: arguments,
		},
	}
}

func appendToolCallToLastAssistant(messages []dto.Message, toolCall dto.ToolCallRequest) []dto.Message {
	if len(messages) == 0 || messages[len(messages)-1].Role != "assistant" {
		messages = append(messages, dto.Message{Role: "assistant"})
	}

	idx := len(messages) - 1
	toolCalls := messages[idx].ParseToolCalls()
	toolCalls = append(toolCalls, toolCall)
	toolCallsRaw, _ := common.Marshal(toolCalls)
	messages[idx].ToolCalls = toolCallsRaw
	return messages
}

func responsesRequestToolsToChat(raw json.RawMessage) ([]dto.ToolCallRequest, error) {
	if !rawJSONPresent(raw) {
		return nil, nil
	}

	var tools []map[string]any
	if err := common.Unmarshal(raw, &tools); err != nil {
		return nil, fmt.Errorf("invalid tools: %w", err)
	}

	return responsesToolMapsToChat(tools), nil
}

func responsesToolMapsToChat(tools []map[string]any) []dto.ToolCallRequest {
	out := make([]dto.ToolCallRequest, 0, len(tools))
	for _, tool := range tools {
		toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
		if toolType == "function" {
			if converted, ok := responsesFunctionToolToChat(tool); ok {
				out = append(out, converted)
			}
			continue
		}
		if specialTool := responsesSpecialToolToChatFunction(toolType); specialTool != nil {
			out = append(out, *specialTool)
			continue
		}
		out = append(out, responsesNonFunctionToolToChatFunctions(tool)...)
	}
	return out
}

func responsesToolSearchOutputToolsToChat(rawInput json.RawMessage) []dto.ToolCallRequest {
	if !rawJSONPresent(rawInput) || common.GetJsonType(rawInput) != "array" {
		return nil
	}
	var items []map[string]any
	if err := common.Unmarshal(rawInput, &items); err != nil {
		return nil
	}
	out := make([]dto.ToolCallRequest, 0)
	for _, item := range items {
		if strings.TrimSpace(common.Interface2String(item["type"])) != responsesInputTypeToolSearchOutput {
			continue
		}
		rawTools, ok := item["tools"].([]any)
		if !ok {
			continue
		}
		tools := make([]map[string]any, 0, len(rawTools))
		for _, rawTool := range rawTools {
			if tool, ok := rawTool.(map[string]any); ok {
				tools = append(tools, tool)
			}
		}
		out = append(out, responsesToolMapsToChat(tools)...)
	}
	return out
}

func responsesFunctionToolToChat(tool map[string]any) (dto.ToolCallRequest, bool) {
	if function, ok := tool["function"].(map[string]any); ok {
		name := strings.TrimSpace(common.Interface2String(function["name"]))
		if name == "" {
			return dto.ToolCallRequest{}, false
		}
		return dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        name,
				Description: common.Interface2String(function["description"]),
				Parameters:  normalizeChatToolParameters(firstPresent(function, "parameters", "input_schema", "schema"), defaultChatToolParameters()),
			},
		}, true
	}
	name := strings.TrimSpace(common.Interface2String(tool["name"]))
	if name == "" {
		return dto.ToolCallRequest{}, false
	}
	return dto.ToolCallRequest{
		Type: "function",
		Function: dto.FunctionRequest{
			Name:        name,
			Description: common.Interface2String(tool["description"]),
			Parameters:  normalizeChatToolParameters(firstPresent(tool, "parameters", "input_schema", "schema"), defaultChatToolParameters()),
		},
	}, true
}

func appendChatToolsUnique(base []dto.ToolCallRequest, extra ...dto.ToolCallRequest) []dto.ToolCallRequest {
	if len(extra) == 0 {
		return base
	}
	seen := make(map[string]bool, len(base)+len(extra))
	for _, tool := range base {
		seen[chatToolDedupKey(tool)] = true
	}
	out := append([]dto.ToolCallRequest{}, base...)
	for _, tool := range extra {
		key := chatToolDedupKey(tool)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, tool)
	}
	return out
}

func chatToolDedupKey(tool dto.ToolCallRequest) string {
	if tool.Type == "function" {
		return "function:" + strings.TrimSpace(tool.Function.Name)
	}
	return strings.TrimSpace(tool.Type)
}

func normalizeChatCompletionTools(tools []dto.ToolCallRequest) []dto.ToolCallRequest {
	if len(tools) == 0 {
		return tools
	}
	out := make([]dto.ToolCallRequest, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Function.Name)
		if name == "" {
			name = strings.TrimSpace(tool.Type)
		}
		if name == "" {
			name = dto.CustomType
		}
		tool.Type = "function"
		tool.Function.Name = sanitizeChatCompatFunctionNamePart(name)
		tool.Function.Parameters = normalizeChatToolParameters(tool.Function.Parameters, defaultChatToolParameters())
		tool.Custom = nil
		out = append(out, tool)
	}
	return out
}

func SanitizeChatCompletionsToolsJSON(data []byte) ([]byte, bool, error) {
	var body map[string]any
	if err := common.Unmarshal(data, &body); err != nil {
		return nil, false, err
	}
	rawTools, ok := body["tools"].([]any)
	if !ok || len(rawTools) == 0 {
		return data, false, nil
	}

	toolMaps := make([]map[string]any, 0, len(rawTools))
	for _, rawTool := range rawTools {
		if tool, ok := rawTool.(map[string]any); ok {
			toolMaps = append(toolMaps, tool)
			continue
		}
		toolMaps = append(toolMaps, map[string]any{
			"type": common.Interface2String(rawTool),
		})
	}

	tools := normalizeChatCompletionTools(responsesToolMapsToChat(toolMaps))
	toolsRaw, err := common.Marshal(tools)
	if err != nil {
		return nil, false, err
	}
	var normalizedTools any
	if err := common.Unmarshal(toolsRaw, &normalizedTools); err != nil {
		return nil, false, err
	}
	body["tools"] = normalizedTools
	out, err := common.Marshal(body)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

func SanitizeResponsesToolsJSON(data []byte) ([]byte, bool, error) {
	var body map[string]any
	if err := common.Unmarshal(data, &body); err != nil {
		return nil, false, err
	}
	rawTools, ok := body["tools"].([]any)
	if !ok || len(rawTools) == 0 {
		return data, false, nil
	}

	toolMaps := make([]map[string]any, 0, len(rawTools))
	for _, rawTool := range rawTools {
		if tool, ok := rawTool.(map[string]any); ok {
			toolMaps = append(toolMaps, tool)
			continue
		}
		toolMaps = append(toolMaps, map[string]any{
			"type": common.Interface2String(rawTool),
		})
	}

	chatTools := normalizeChatCompletionTools(responsesToolMapsToChat(toolMaps))
	normalizedTools := make([]map[string]any, 0, len(chatTools))
	for _, tool := range chatTools {
		responseTool := map[string]any{
			"type":       "function",
			"name":       tool.Function.Name,
			"parameters": tool.Function.Parameters,
		}
		if description := strings.TrimSpace(tool.Function.Description); description != "" {
			responseTool["description"] = description
		}
		normalizedTools = append(normalizedTools, responseTool)
	}
	body["tools"] = normalizedTools
	out, err := common.Marshal(body)
	if err != nil {
		return nil, false, err
	}
	return out, true, nil
}

func firstPresent(values map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return value
		}
	}
	return nil
}

// responsesSpecialToolToChatFunction converts known Responses API server-side
// tools (tool_search, spawn_agent, etc.) into well-defined Chat Completions
// function tools, mirroring how CC Switch adapts them for third-party models.
// Returns nil for tool types that should fall through to the generic wrapper.
func responsesSpecialToolToChatFunction(toolType string) *dto.ToolCallRequest {
	switch strings.TrimSpace(toolType) {
	case "tool_search":
		return &dto.ToolCallRequest{
			Type: "function",
			Function: dto.FunctionRequest{
				Name:        "tool_search",
				Description: "Search and load Codex tools, plugins, connectors, and MCP namespaces for the current task.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Search query for tools or connectors to load.",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Maximum number of tool groups to return.",
						},
					},
					"required": []string{"query"},
				},
			},
		}
	default:
		return nil
	}
}

func responsesNonFunctionToolToChatFunctions(tool map[string]any) []dto.ToolCallRequest {
	toolType := strings.TrimSpace(common.Interface2String(tool["type"]))
	if toolType == "" {
		toolType = dto.CustomType
	}
	toolName := strings.TrimSpace(common.Interface2String(tool["name"]))
	if toolName == "" {
		toolName = strings.TrimSpace(common.Interface2String(tool["namespace"]))
	}
	if toolName == "" {
		toolName = toolType
	}

	if namespaceTools := responsesNamespaceToolDefinitions(tool); len(namespaceTools) > 0 {
		out := make([]dto.ToolCallRequest, 0, len(namespaceTools))
		for _, namespaceTool := range namespaceTools {
			childName := strings.TrimSpace(common.Interface2String(namespaceTool["name"]))
			if childName == "" {
				continue
			}
			wrapped := cloneMap(namespaceTool)
			wrapped[chatCompatWrappedToolOriginalTypeKey] = toolType
			wrapped[chatCompatWrappedToolOriginalNameKey] = childName
			out = append(out, dto.ToolCallRequest{
				Type: "function",
				Function: dto.FunctionRequest{
					Name:        chatCompatWrappedToolName(toolName, childName),
					Description: responsesToolDescription(namespaceTool, tool),
					Parameters:  responsesToolParameters(wrapped, true),
				},
			})
		}
		if len(out) > 0 {
			return out
		}
	}

	wrapped := cloneMap(tool)
	wrapped[chatCompatWrappedToolOriginalTypeKey] = toolType
	wrapped[chatCompatWrappedToolOriginalNameKey] = toolName
	return []dto.ToolCallRequest{{
		Type: "function",
		Function: dto.FunctionRequest{
			Name:        chatCompatWrappedToolName(toolType, toolName),
			Description: responsesToolDescription(tool, nil),
			Parameters:  responsesToolParameters(wrapped, false),
		},
	}}
}

func responsesNamespaceToolDefinitions(tool map[string]any) []map[string]any {
	for _, key := range []string{"tools", "functions"} {
		items, ok := tool[key].([]any)
		if !ok {
			continue
		}
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

func responsesToolDescription(tool map[string]any, fallback map[string]any) string {
	if description := strings.TrimSpace(common.Interface2String(tool["description"])); description != "" {
		return description
	}
	if fallback != nil {
		return strings.TrimSpace(common.Interface2String(fallback["description"]))
	}
	return ""
}

func responsesToolParameters(tool map[string]any, allowOriginalSchema bool) any {
	if allowOriginalSchema {
		for _, key := range []string{"parameters", "input_schema", "schema"} {
			if parameters, ok := tool[key]; ok {
				return normalizeChatToolParameters(parameters, defaultChatToolParameters())
			}
		}
		return defaultChatToolParameters()
	}
	return wrappedChatToolParameters()
}

func normalizeChatToolParameters(parameters any, fallback map[string]any) any {
	schema, ok := parameters.(map[string]any)
	if !ok || schema == nil {
		return fallback
	}
	out := cloneMap(schema)
	if strings.TrimSpace(common.Interface2String(out["type"])) != "object" {
		out["type"] = "object"
	}
	if _, ok := out["properties"]; !ok {
		out["properties"] = map[string]any{}
	}
	return out
}

func defaultChatToolParameters() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties":           map[string]any{},
	}
}

func wrappedChatToolParameters() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties": map[string]any{
			chatCompatWrappedToolPayloadKey: map[string]any{
				"description": "Payload for the original Responses API tool.",
			},
		},
	}
}

func chatCompatWrappedToolName(toolType string, toolName string) string {
	sanitizedType := sanitizeChatCompatFunctionNamePart(toolType)
	sanitizedName := sanitizeChatCompatFunctionNamePart(toolName)
	name := chatCompatWrappedToolPrefix + sanitizedType
	if sanitizedName != "" && sanitizedName != sanitizedType {
		name += "__" + sanitizedName
	}
	if len(name) > 64 {
		name = name[:64]
	}
	return strings.TrimRight(name, "_-")
}

func sanitizeChatCompatFunctionNamePart(value string) string {
	value = strings.TrimSpace(value)
	value = chatCompatFunctionNameInvalidChars.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_-")
	if value == "" {
		return "tool"
	}
	return value
}

func cloneMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src)+2)
	for key, value := range src {
		out[key] = value
	}
	return out
}

func responsesRequestToolChoiceToChat(raw json.RawMessage) (any, error) {
	if !rawJSONPresent(raw) {
		return nil, nil
	}
	if common.GetJsonType(raw) == "string" {
		var choice string
		if err := common.Unmarshal(raw, &choice); err != nil {
			return nil, fmt.Errorf("invalid tool_choice: %w", err)
		}
		return choice, nil
	}

	var choice map[string]any
	if err := common.Unmarshal(raw, &choice); err != nil {
		return nil, fmt.Errorf("invalid tool_choice: %w", err)
	}
	if common.Interface2String(choice["type"]) == "function" {
		name := strings.TrimSpace(common.Interface2String(choice["name"]))
		if name != "" {
			return map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": name,
				},
			}, nil
		}
	}
	return choice, nil
}

func responsesRequestTextToChatResponseFormat(raw json.RawMessage) (*dto.ResponseFormat, error) {
	if !rawJSONPresent(raw) {
		return nil, nil
	}

	var textConfig map[string]any
	if err := common.Unmarshal(raw, &textConfig); err != nil {
		return nil, fmt.Errorf("invalid text config: %w", err)
	}
	format, ok := textConfig["format"].(map[string]any)
	if !ok {
		return nil, nil
	}

	formatType := strings.TrimSpace(common.Interface2String(format["type"]))
	if formatType == "" {
		return nil, nil
	}

	out := &dto.ResponseFormat{Type: formatType}
	if formatType == "json_schema" {
		schemaRaw, err := common.Marshal(format)
		if err != nil {
			return nil, err
		}
		out.JsonSchema = schemaRaw
	}
	return out, nil
}

func responsesImagePartToChatImageURL(part map[string]any) any {
	if imageURL, ok := part["image_url"]; ok {
		return imageURL
	}
	imageURL := map[string]any{}
	for _, key := range []string{"url", "file_id", "detail"} {
		if value, ok := part[key]; ok {
			imageURL[key] = value
		}
	}
	if len(imageURL) == 0 {
		return part
	}
	return imageURL
}

func responsesFilePartToChatFile(part map[string]any) any {
	if file, ok := part["file"]; ok {
		return file
	}
	file := map[string]any{}
	for _, key := range []string{"file_id", "file_data", "filename", "file_url"} {
		if value, ok := part[key]; ok {
			file[key] = value
		}
	}
	if len(file) == 0 {
		return part
	}
	return file
}

func responsesVideoPartToChatVideoURL(part map[string]any) any {
	if videoURL, ok := part["video_url"]; ok {
		if videoURLMap, ok := videoURL.(map[string]any); ok {
			if url := common.Interface2String(videoURLMap["url"]); url != "" {
				return url
			}
		}
		return videoURL
	}
	if url := common.Interface2String(part["url"]); url != "" {
		return url
	}
	return responsesPartPayload(part, "video_url")
}

func responsesPartPayload(part map[string]any, key string) any {
	if value, ok := part[key]; ok {
		return value
	}
	payload := make(map[string]any, len(part))
	for k, value := range part {
		if k == "type" {
			continue
		}
		payload[k] = value
	}
	return payload
}

func responsesCallID(item map[string]any) string {
	callID := strings.TrimSpace(common.Interface2String(item["call_id"]))
	if callID != "" {
		return callID
	}
	return strings.TrimSpace(common.Interface2String(item["id"]))
}

func responsesArgumentsString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		raw, err := common.Marshal(v)
		if err != nil {
			return common.Interface2String(v)
		}
		return string(raw)
	}
}

// responsesReasoningItemToContent extracts the textual reasoning from a
// Responses API reasoning input item. Codex passes previous reasoning back as
// items like {"type":"reasoning","summary":[{"type":"summary_text","text":"..."}]}.
// Providers in thinking mode (e.g. DeepSeek) require this to come back as
// assistant message `reasoning_content`, otherwise they reject the request.
func responsesReasoningItemToContent(item map[string]any) string {
	// Prefer the official Responses reasoning summary parts, then tolerate
	// older compatibility payloads that used content.
	for _, key := range []string{"summary", "content"} {
		parts, ok := item[key].([]any)
		if !ok {
			continue
		}
		var sb strings.Builder
		for _, raw := range parts {
			part, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			partType := strings.TrimSpace(common.Interface2String(part["type"]))
			if partType != "summary_text" && partType != "text" && partType != "output_text" && partType != "input_text" {
				continue
			}
			if text := strings.TrimSpace(common.Interface2String(part["text"])); text != "" {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString(text)
			}
		}
		if sb.Len() > 0 {
			return sb.String()
		}
	}
	// Some providers carry an opaque encrypted_content blob that must be echoed
	// back verbatim; fall back to it when there is no readable summary.
	if enc := strings.TrimSpace(common.Interface2String(item["encrypted_content"])); enc != "" {
		return enc
	}
	return ""
}

func responseToolOutputToChatContent(value any) any {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		raw, err := common.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(raw)
	}
}

func responsesJSONString(raw json.RawMessage) (string, error) {
	if common.GetJsonType(raw) != "string" {
		return string(raw), nil
	}
	var value string
	if err := common.Unmarshal(raw, &value); err != nil {
		return "", err
	}
	return value, nil
}

func rawJSONPresent(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	return common.GetJsonType(raw) != "null"
}
