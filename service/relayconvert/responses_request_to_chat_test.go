package relayconvert

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestResponsesRequestToChatCompletionsRequestInstructionsAndScalarInput(t *testing.T) {
	stream := true
	temperature := 0.0
	topP := 0.9
	maxOutputTokens := uint(128)

	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model:                "gpt-test",
		Instructions:         mustRawMessage(t, "system rules"),
		Input:                mustRawMessage(t, "hello"),
		Stream:               &stream,
		StreamOptions:        &dto.StreamOptions{IncludeUsage: true},
		MaxOutputTokens:      &maxOutputTokens,
		Temperature:          &temperature,
		TopP:                 &topP,
		User:                 mustRawMessage(t, "user-1"),
		Store:                mustRawMessage(t, false),
		Metadata:             mustRawMessage(t, map[string]any{"trace": "abc"}),
		PromptCacheKey:       mustRawMessage(t, "cache-key"),
		PromptCacheRetention: mustRawMessage(t, "24h"),
		Reasoning:            &dto.Reasoning{Effort: "medium"},
	})
	require.NoError(t, err)

	assert.Equal(t, "gpt-test", got.Model)
	require.Len(t, got.Messages, 2)
	assert.Equal(t, dto.Message{Role: "system", Content: "system rules"}, got.Messages[0])
	assert.Equal(t, dto.Message{Role: "user", Content: "hello"}, got.Messages[1])
	assert.Same(t, &stream, got.Stream)
	require.NotNil(t, got.StreamOptions)
	assert.True(t, got.StreamOptions.IncludeUsage)
	assert.Equal(t, maxOutputTokens, lo.FromPtr(got.MaxCompletionTokens))
	assert.Equal(t, 0.0, lo.FromPtr(got.Temperature))
	assert.Equal(t, 0.9, lo.FromPtr(got.TopP))
	assert.Equal(t, "cache-key", got.PromptCacheKey)
	assert.Equal(t, "medium", got.ReasoningEffort)
	assert.Equal(t, `"user-1"`, string(got.User))
	assert.Equal(t, `false`, string(got.Store))
	assert.Equal(t, "abc", gjson.GetBytes(got.Metadata, "trace").String())
}

func TestResponsesRequestToChatCompletionsRequestMultimodalInput(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "input_text", "text": "look"},
					{"type": "input_image", "image_url": "https://example.test/a.png", "detail": "low"},
					{"type": "input_file", "file_id": "file_1", "filename": "a.txt"},
					{"type": "input_audio", "input_audio": map[string]any{"data": "abc", "format": "wav"}},
					{"type": "input_video", "video_url": map[string]any{"url": "https://example.test/v.mp4"}},
				},
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 1)
	assert.Equal(t, "user", got.Messages[0].Role)
	parts := got.Messages[0].ParseContent()
	require.Len(t, parts, 5)
	assert.Equal(t, dto.ContentTypeText, parts[0].Type)
	assert.Equal(t, "look", parts[0].Text)
	assert.Equal(t, dto.ContentTypeImageURL, parts[1].Type)
	assert.Equal(t, "https://example.test/a.png", parts[1].GetImageMedia().Url)
	assert.Equal(t, dto.ContentTypeFile, parts[2].Type)
	assert.Equal(t, "file_1", parts[2].GetFile().FileId)
	assert.Equal(t, dto.ContentTypeInputAudio, parts[3].Type)
	assert.Equal(t, "wav", parts[3].GetInputAudio().Format)
	assert.Equal(t, dto.ContentTypeVideoUrl, parts[4].Type)
	assert.Equal(t, "https://example.test/v.mp4", parts[4].GetVideoUrl().Url)
}

func TestResponsesRequestToChatCompletionsRequestAssistantTextAndFunctionCallCoexist(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "output_text", "text": "I will call."},
				},
			},
			{
				"type":      "function_call",
				"call_id":   "call_1",
				"name":      "lookup",
				"arguments": map[string]any{"q": "x"},
			},
			{
				"type":    "function_call_output",
				"call_id": "call_1",
				"output":  map[string]any{"ok": true},
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 2)
	assert.Equal(t, "assistant", got.Messages[0].Role)
	assert.Equal(t, "I will call.", got.Messages[0].StringContent())
	toolCalls := got.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_1", toolCalls[0].ID)
	assert.Equal(t, "function", toolCalls[0].Type)
	assert.Equal(t, "lookup", toolCalls[0].Function.Name)
	assert.JSONEq(t, `{"q":"x"}`, toolCalls[0].Function.Arguments)
	assert.Equal(t, "tool", got.Messages[1].Role)
	assert.Equal(t, "call_1", got.Messages[1].ToolCallId)
	assert.JSONEq(t, `{"ok":true}`, got.Messages[1].StringContent())
}

func TestResponsesRequestToChatCompletionsRequestOnlyFunctionCallCreatesAssistant(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{
				"type":      "function_call",
				"call_id":   "call_1",
				"name":      "lookup",
				"arguments": `{"q":"x"}`,
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 1)
	assert.Equal(t, "assistant", got.Messages[0].Role)
	assert.Nil(t, got.Messages[0].Content)
	toolCalls := got.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, `{"q":"x"}`, toolCalls[0].Function.Arguments)
}

func TestResponsesRequestToChatCompletionsRequestToolsToolChoiceAndTextFormat(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, "hello"),
		Tools: mustRawMessage(t, []map[string]any{
			{
				"type":        "function",
				"name":        "lookup",
				"description": "Lookup data",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"q": map[string]any{"type": "string"},
					},
				},
			},
		}),
		ToolChoice: mustRawMessage(t, map[string]any{
			"type": "function",
			"name": "lookup",
		}),
		Text: mustRawMessage(t, map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "answer",
				"schema": map[string]any{"type": "object"},
				"strict": true,
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Tools, 1)
	assert.Equal(t, "function", got.Tools[0].Type)
	assert.Equal(t, "lookup", got.Tools[0].Function.Name)
	assert.Equal(t, "Lookup data", got.Tools[0].Function.Description)
	assert.Equal(t, "object", got.Tools[0].Function.Parameters.(map[string]any)["type"])
	assert.Equal(t, map[string]any{
		"type": "function",
		"function": map[string]any{
			"name": "lookup",
		},
	}, got.ToolChoice)
	require.NotNil(t, got.ResponseFormat)
	assert.Equal(t, "json_schema", got.ResponseFormat.Type)
	assert.Equal(t, "answer", gjson.GetBytes(got.ResponseFormat.JsonSchema, "name").String())
	assert.True(t, gjson.GetBytes(got.ResponseFormat.JsonSchema, "strict").Bool())
}

func TestResponsesRequestToChatCompletionsRequestWrapsNamespaceTools(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test-max",
		Input: mustRawMessage(t, "hello"),
		Tools: mustRawMessage(t, []map[string]any{
			{
				"type":      "namespace",
				"namespace": "shell",
				"tools": []map[string]any{
					{
						"name":        "exec_command",
						"description": "Run a command",
						"input_schema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"cmd": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		}),
	})
	require.NoError(t, err)

	assert.Equal(t, "gpt-test", got.Model)
	assert.Equal(t, "max", got.ReasoningEffort)
	require.Len(t, got.Tools, 1)
	assert.Equal(t, "function", got.Tools[0].Type)
	assert.Equal(t, "codex__shell__exec_command", got.Tools[0].Function.Name)
	assert.Equal(t, "Run a command", got.Tools[0].Function.Description)
	assert.Equal(t, "object", got.Tools[0].Function.Parameters.(map[string]any)["type"])
}

func TestNormalizeCodexChatReasoningEffort(t *testing.T) {
	tests := []struct {
		name   string
		effort string
		mode   string
		want   string
	}{
		{name: "openrouter max clamps to xhigh", effort: "max", mode: "openrouter", want: "xhigh"},
		{name: "openrouter medium passthrough", effort: "medium", mode: "openrouter", want: "medium"},
		{name: "deepseek max", effort: "xhigh", mode: "deepseek", want: "max"},
		{name: "deepseek lower levels use high", effort: "low", mode: "deepseek", want: "high"},
		{name: "low high minimal", effort: "minimal", mode: "low_high", want: "low"},
		{name: "low high max", effort: "max", mode: "low_high", want: "high"},
		{name: "none omitted", effort: "none", mode: "openrouter", want: ""},
		{name: "passthrough max", effort: "max", mode: "", want: "max"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeCodexChatReasoningEffort(tt.effort, tt.mode))
		})
	}
}

func TestResponsesRequestToChatCompletionsRequestCustomToolCallPreservesRawShape(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{
				"type":    "custom_tool_call",
				"call_id": "call_custom",
				"name":    "apply_patch",
				"input":   "patch body",
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Messages, 1)
	toolCalls := got.Messages[0].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "function", toolCalls[0].Type)
	assert.Equal(t, "call_custom", toolCalls[0].ID)
	assert.Equal(t, "codex__custom_tool_call__apply_patch", toolCalls[0].Function.Name)
	assert.Equal(t, "patch body", toolCalls[0].Function.Arguments)
}

func TestResponsesRequestToChatCompletionsRequestConvertsToolSearch(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{"role": "user", "content": "search for tools"},
		}),
		Tools: mustRawMessage(t, []map[string]any{
			{"type": "tool_search"},
			{"type": "function", "name": "read_file", "description": "read", "parameters": map[string]any{"type": "object"}},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Tools, 2)

	// tool_search should become a well-defined function tool
	assert.Equal(t, "function", got.Tools[0].Type)
	assert.Equal(t, "tool_search", got.Tools[0].Function.Name)
	assert.Contains(t, got.Tools[0].Function.Description, "Search and load Codex tools")

	// regular function tool should pass through unchanged
	assert.Equal(t, "read_file", got.Tools[1].Function.Name)
}

func TestResponsesRequestToChatCompletionsRequestNormalizesUnsupportedCodexToolsForChat(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, "use tools"),
		Tools: mustRawMessage(t, []map[string]any{
			{"type": "tool_search"},
			{
				"type":      "namespace",
				"namespace": "shell",
				"tools": []map[string]any{
					{"name": "exec_command", "input_schema": map[string]any{"type": "object"}},
				},
			},
			{
				"type":        "custom_tool",
				"name":        "apply_patch",
				"description": "Apply a patch.",
			},
			{
				"type": "mcp",
				"name": "browser.click",
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Tools, 4)
	body, err := common.Marshal(got)
	require.NoError(t, err)
	for i, tool := range got.Tools {
		assert.Equal(t, "function", tool.Type)
		assert.NotEmpty(t, tool.Function.Name)
		assert.Equal(t, "function", gjson.GetBytes(body, fmt.Sprintf("tools.%d.type", i)).String())
		assert.NotEmpty(t, gjson.GetBytes(body, fmt.Sprintf("tools.%d.function.name", i)).String())
		assert.Equal(t, "object", gjson.GetBytes(body, fmt.Sprintf("tools.%d.function.parameters.type", i)).String())
	}
	assert.Equal(t, "tool_search", got.Tools[0].Function.Name)
	assert.Equal(t, "codex__shell__exec_command", got.Tools[1].Function.Name)
	assert.Equal(t, "codex__custom_tool__apply_patch", got.Tools[2].Function.Name)
	assert.Equal(t, "codex__mcp__browser_click", got.Tools[3].Function.Name)
}

func TestSanitizeChatCompletionsToolsJSONNormalizesRawResponsesTools(t *testing.T) {
	body := mustRawMessage(t, map[string]any{
		"model": "glm-5.2",
		"messages": []map[string]any{
			{"role": "user", "content": "hi"},
		},
		"tools": []map[string]any{
			{"type": "tool_search"},
			{
				"type":      "namespace",
				"namespace": "shell",
				"tools": []map[string]any{
					{"name": "exec_command"},
				},
			},
			{"type": "mcp", "name": "browser.click"},
		},
	})

	sanitized, changed, err := SanitizeChatCompletionsToolsJSON(body)
	require.NoError(t, err)
	require.True(t, changed)

	assert.Equal(t, "glm-5.2", gjson.GetBytes(sanitized, "model").String())
	for i := 0; i < 3; i++ {
		assert.Equal(t, "function", gjson.GetBytes(sanitized, fmt.Sprintf("tools.%d.type", i)).String())
		assert.Equal(t, "object", gjson.GetBytes(sanitized, fmt.Sprintf("tools.%d.function.parameters.type", i)).String())
	}
	assert.Equal(t, "tool_search", gjson.GetBytes(sanitized, "tools.0.function.name").String())
	assert.Equal(t, "codex__shell__exec_command", gjson.GetBytes(sanitized, "tools.1.function.name").String())
	assert.Equal(t, "codex__mcp__browser_click", gjson.GetBytes(sanitized, "tools.2.function.name").String())
}

func TestResponsesRequestToChatCompletionsRequestHandlesToolSearchCallInput(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{"role": "user", "content": "hi"},
			{
				"type":    "tool_search_call",
				"call_id": "call_ts_1",
				"name":    "tool_search",
				"input":   map[string]any{"query": "file ops", "limit": 5},
			},
			{
				"type":    "tool_search_output",
				"call_id": "call_ts_1",
				"output":  "found 5 tools",
			},
		}),
	})
	require.NoError(t, err)

	// Should produce: user message, assistant with tool call, tool result
	require.Len(t, got.Messages, 3)
	assert.Equal(t, "user", got.Messages[0].Role)
	assert.Equal(t, "assistant", got.Messages[1].Role)

	toolCalls := got.Messages[1].ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_ts_1", toolCalls[0].ID)
	assert.Equal(t, "tool_search", toolCalls[0].Function.Name)
	assert.Equal(t, "file ops", gjson.Get(toolCalls[0].Function.Arguments, "query").String())
	assert.Equal(t, int64(5), gjson.Get(toolCalls[0].Function.Arguments, "limit").Int())

	assert.Equal(t, "tool", got.Messages[2].Role)
	assert.Equal(t, "call_ts_1", got.Messages[2].ToolCallId)
	assert.Equal(t, "found 5 tools", got.Messages[2].StringContent())
}

func TestResponsesRequestToChatCompletionsRequestToolSearchOutputProvidesTools(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model:      "gpt-test",
		ToolChoice: mustRawMessage(t, "auto"),
		Input: mustRawMessage(t, []map[string]any{
			{
				"type":      "tool_search_output",
				"call_id":   "call_ts_1",
				"status":    "completed",
				"execution": "client",
				"tools": []map[string]any{
					{
						"type":        "function",
						"name":        "search_docs",
						"description": "Search documentation.",
						"parameters": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"query": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Tools, 1)
	assert.Equal(t, "search_docs", got.Tools[0].Function.Name)
	assert.Equal(t, "Search documentation.", got.Tools[0].Function.Description)
	assert.Equal(t, "auto", got.ToolChoice)
}

func TestResponsesRequestToChatCompletionsRequestNormalizesToolSearchOutputToolSchemas(t *testing.T) {
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, []map[string]any{
			{
				"type":    "tool_search_output",
				"call_id": "call_ts_1",
				"tools": []map[string]any{
					{
						"type": "namespace",
						"name": "automation",
						"tools": []map[string]any{
							{
								"name":         "automation_update",
								"description":  "Update automations.",
								"input_schema": map[string]any{"type": nil},
							},
						},
					},
				},
			},
		}),
	})
	require.NoError(t, err)

	require.Len(t, got.Tools, 1)
	assert.Equal(t, "codex__automation__automation_update", got.Tools[0].Function.Name)
	params, ok := got.Tools[0].Function.Parameters.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "object", params["type"])
	assert.NotNil(t, params["properties"])
}

func TestResponsesRequestToChatCompletionsRequestDropsToolChoiceWithoutTools(t *testing.T) {
	parallel := true
	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model:             "gpt-test",
		Input:             mustRawMessage(t, "hello"),
		ToolChoice:        mustRawMessage(t, "auto"),
		ParallelToolCalls: mustRawMessage(t, parallel),
	})
	require.NoError(t, err)

	assert.Empty(t, got.Tools)
	assert.Nil(t, got.ToolChoice)
	assert.Nil(t, got.ParallelTooCalls)
}

func TestChatCompletionsResponseToResponsesMapsToolSearchCall(t *testing.T) {
	chat := &dto.OpenAITextResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message:      assistantMessageWithTool("", "call_1", "tool_search", `{"query":"x","limit":3}`),
				FinishReason: "tool_calls",
			},
		},
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(chat, "resp_1")
	require.NoError(t, err)

	require.Len(t, resp.Output, 1)
	assert.Equal(t, "tool_search_call", resp.Output[0].Type)
	assert.Empty(t, resp.Output[0].Name)
	assert.Equal(t, "client", resp.Output[0].Execution)
	assert.Equal(t, "x", gjson.GetBytes(resp.Output[0].Arguments, "query").String())
	assert.Equal(t, int64(3), gjson.GetBytes(resp.Output[0].Arguments, "limit").Int())
}

func TestResponsesRequestToChatCompletionsRequestRejectsStatefulFields(t *testing.T) {
	tests := []struct {
		name string
		req  *dto.OpenAIResponsesRequest
		want string
	}{
		{
			name: "conversation",
			req:  &dto.OpenAIResponsesRequest{Model: "gpt-test", Conversation: mustRawMessage(t, "conv_1")},
			want: "conversation",
		},
		{
			name: "previous response",
			req:  &dto.OpenAIResponsesRequest{Model: "gpt-test", PreviousResponseID: "resp_1"},
			want: "previous_response_id",
		},
		{
			name: "prompt",
			req:  &dto.OpenAIResponsesRequest{Model: "gpt-test", Prompt: mustRawMessage(t, map[string]any{"id": "pmpt_1"})},
			want: "prompt",
		},
		{
			name: "context management",
			req:  &dto.OpenAIResponsesRequest{Model: "gpt-test", ContextManagement: mustRawMessage(t, map[string]any{"type": "auto"})},
			want: "context_management",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ResponsesRequestToChatCompletionsRequest(tt.req)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
			assert.Contains(t, err.Error(), "stateful fields")
		})
	}
}

func mustRawMessage(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := common.Marshal(value)
	require.NoError(t, err)
	return raw
}

func TestResponsesRequestToChatCompletionsRequestPreservesReasoningInput(t *testing.T) {
	// Simulates Codex sending back the previous reasoning summary before the
	// assistant text turn. DeepSeek/GLM thinking mode requires this to come
	// back as assistant `reasoning_content`.
	input := []map[string]any{
		{
			"type": "reasoning",
			"id":   "rs_abc",
			"content": []map[string]any{
				{"type": "summary_text", "text": "Let me think about this."},
			},
		},
		{
			"type":    "message",
			"role":    "assistant",
			"content": "Here is my answer.",
		},
		{
			"type":    "message",
			"role":    "user",
			"content": "Follow up question",
		},
	}

	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, input),
	})
	require.NoError(t, err)

	// Reasoning and the assistant text are one Chat Completions message.
	require.Len(t, got.Messages, 2, "assistant + user")
	assert.Equal(t, "assistant", got.Messages[0].Role)
	require.NotNil(t, got.Messages[0].ReasoningContent)
	assert.Equal(t, "Let me think about this.", *got.Messages[0].ReasoningContent)
	assert.Equal(t, "Here is my answer.", got.Messages[0].StringContent())
	assert.Equal(t, "user", got.Messages[1].Role)
}

func TestResponsesRequestToChatCompletionsRequestReasoningBeforeAssistantOnly(t *testing.T) {
	// When a reasoning item appears with no following assistant message yet,
	// an assistant message is created to host reasoning_content.
	input := []map[string]any{
		{
			"type": "reasoning",
			"content": []map[string]any{
				{"type": "summary_text", "text": "standalone reasoning"},
			},
		},
	}

	got, err := ResponsesRequestToChatCompletionsRequest(&dto.OpenAIResponsesRequest{
		Model: "gpt-test",
		Input: mustRawMessage(t, input),
	})
	require.NoError(t, err)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, "assistant", got.Messages[0].Role)
	require.NotNil(t, got.Messages[0].ReasoningContent)
	assert.Equal(t, "standalone reasoning", *got.Messages[0].ReasoningContent)
}
