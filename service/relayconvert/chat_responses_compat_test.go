package relayconvert

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestChatCompletionsRequestToResponsesRequestInstructionsAndTools(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-test",
		N:     lo.ToPtr(1),
		Messages: []dto.Message{
			{Role: "system", Content: "system rules"},
			{Role: "developer", Content: "developer rules"},
			{Role: "user", Content: []any{
				map[string]any{"type": "text", "text": "look"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://example.test/a.png"}},
			}},
			assistantMessageWithTool("partial text", "call_1", "lookup", `{"q":"x"}`),
			{Role: "tool", ToolCallId: "call_1", Content: "tool result"},
		},
	}

	got, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	assert.Equal(t, "gpt-test", got.Model)
	assert.Equal(t, `"system rules\n\ndeveloper rules"`, string(got.Instructions))
	assert.Equal(t, "input_image", gjson.GetBytes(got.Input, "0.content.1.type").String())
	assert.Equal(t, "function_call", gjson.GetBytes(got.Input, "2.type").String())
	assert.Equal(t, "call_1", gjson.GetBytes(got.Input, "2.call_id").String())
	assert.Equal(t, "function_call_output", gjson.GetBytes(got.Input, "3.type").String())
}

func TestChatCompletionsRequestToResponsesPreservesReasoningContent(t *testing.T) {
	reasoning := "I should think about this carefully"
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-test",
		N:     lo.ToPtr(1),
		Messages: []dto.Message{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "answer", ReasoningContent: &reasoning},
			{Role: "user", Content: "follow up"},
		},
	}

	got, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	// input: [0] user "hi", [1] reasoning, [2] assistant "answer", [3] user "follow up"
	assert.Equal(t, "reasoning", gjson.GetBytes(got.Input, "1.type").String())
	assert.Equal(t, reasoning, gjson.GetBytes(got.Input, "1.summary.0.text").String())
	assert.False(t, gjson.GetBytes(got.Input, "1.content").Exists())
	assert.Equal(t, "assistant", gjson.GetBytes(got.Input, "2.role").String())
	assert.Equal(t, "answer", gjson.GetBytes(got.Input, "2.content").String())
	assert.Equal(t, "user", gjson.GetBytes(got.Input, "3.role").String())
}

func TestChatCompletionsRequestToResponsesOmitsReasoningWhenEmpty(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-test",
		N:     lo.ToPtr(1),
		Messages: []dto.Message{
			{Role: "user", Content: "hi"},
			{Role: "assistant", Content: "answer"},
		},
	}

	got, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	// No reasoning item should be emitted for non-thinking assistant messages.
	for i := range gjson.GetBytes(got.Input, "#.type").Array() {
		assert.NotEqual(t, "reasoning", gjson.GetBytes(got.Input, fmt.Sprintf("%d.type", i)).String())
	}
}

func TestChatCompletionsRequestToResponsesRequestRejectsMultipleChoices(t *testing.T) {
	_, err := ChatCompletionsRequestToResponsesRequest(&dto.GeneralOpenAIRequest{
		Model: "gpt-test",
		N:     lo.ToPtr(2),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "n>1")
}

func TestResponsesResponseToChatCompletionsPreservesTextAndToolCalls(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		ID:        "resp_1",
		CreatedAt: 123,
		Model:     "gpt-test",
		Status:    []byte(`"completed"`),
		Output: []dto.ResponsesOutput{
			{
				Type: responsesOutputTypeMessage,
				Role: "assistant",
				Content: []dto.ResponsesOutputContent{
					{Type: "output_text", Text: "I will call a tool."},
				},
			},
			{
				Type:      responsesOutputTypeFunctionCall,
				ID:        "fc_1",
				CallId:    "call_1",
				Name:      "lookup",
				Arguments: []byte(`{"q":"x"}`),
			},
		},
		Usage: &dto.Usage{InputTokens: 3, OutputTokens: 4, TotalTokens: 7},
	}

	chat, usage, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_1")
	require.NoError(t, err)
	require.NotNil(t, usage)

	require.Len(t, chat.Choices, 1)
	assert.Equal(t, "tool_calls", chat.Choices[0].FinishReason)
	assert.Equal(t, "I will call a tool.", chat.Choices[0].Message.StringContent())
	toolCalls := chat.Choices[0].Message.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "call_1", toolCalls[0].ID)
	assert.Equal(t, "lookup", toolCalls[0].Function.Name)
	assert.Equal(t, `{"q":"x"}`, toolCalls[0].Function.Arguments)
	assert.Equal(t, 7, usage.TotalTokens)
}

func TestUsageFromChatUsageMapsDeepSeekPromptCacheHitTokens(t *testing.T) {
	usage := UsageFromChatUsage(&dto.Usage{
		PromptTokens:         100,
		CompletionTokens:     20,
		TotalTokens:          120,
		PromptCacheHitTokens: 80,
	})

	require.NotNil(t, usage)
	require.NotNil(t, usage.InputTokensDetails)
	assert.Equal(t, 80, usage.InputTokensDetails.CachedTokens)
	assert.Equal(t, 80, usage.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 100, usage.InputTokens)
	assert.Equal(t, 20, usage.OutputTokens)
	assert.Equal(t, 120, usage.TotalTokens)
}

func TestResponsesResponseToChatCompletionsWrapsNamespacedToolCalls(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		ID:     "resp_1",
		Model:  "gpt-test",
		Status: []byte(`"completed"`),
		Output: []dto.ResponsesOutput{
			{
				Type:      responsesOutputTypeFunctionCall,
				ID:        "fc_1",
				CallId:    "call_1",
				Name:      "spawn_agent",
				Namespace: "multi_agent_v1",
				Arguments: []byte(`{"agent_type":"explore"}`),
			},
		},
	}

	chat, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_1")
	require.NoError(t, err)

	toolCalls := chat.Choices[0].Message.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "codex__multi_agent_v1__spawn_agent", toolCalls[0].Function.Name)
	assert.Equal(t, `{"agent_type":"explore"}`, toolCalls[0].Function.Arguments)
}

func TestResponsesResponseToChatCompletionsPreservesReasoningSummary(t *testing.T) {
	resp := &dto.OpenAIResponsesResponse{
		ID:     "resp_1",
		Model:  "gpt-test",
		Status: []byte(`"completed"`),
		Output: []dto.ResponsesOutput{
			{
				Type: responsesOutputTypeReasoning,
				Content: []dto.ResponsesOutputContent{
					{Type: "summary_text", Text: "first summary"},
					{Type: "summary_text", Text: "\n\nsecond summary"},
				},
			},
			{
				Type: responsesOutputTypeMessage,
				Role: "assistant",
				Content: []dto.ResponsesOutputContent{
					{Type: "output_text", Text: "final"},
				},
			},
		},
	}

	chat, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_1")
	require.NoError(t, err)
	assert.Equal(t, "first summary\n\nsecond summary", chat.Choices[0].Message.GetReasoningContent())
	assert.Equal(t, "final", chat.Choices[0].Message.StringContent())
}

func TestResponsesFinishReasonFromIncompleteStatus(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   string
	}{
		{name: "max output", reason: responsesIncompleteReasonMaxTokens, want: "length"},
		{name: "content filter", reason: responsesIncompleteReasonContentFilter, want: "content_filter"},
		{name: "unknown", reason: "other", want: "length"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ResponsesFinishReasonFromStatus(&dto.OpenAIResponsesResponse{
				Status:            []byte(`"incomplete"`),
				IncompleteDetails: &dto.IncompleteDetails{Reason: tt.reason},
			})
			require.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestResponsesStreamEventToChatChunksUsesOutputIndexForToolArguments(t *testing.T) {
	state := newTestResponsesStreamState()
	outputIndex := 1

	var chunks []dto.ChatCompletionsStreamResponse
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{Type: responsesEventCreated})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{Type: responsesEventOutputTextDelta, Delta: "text before tool"})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:        responsesEventFunctionArgsDelta,
		OutputIndex: &outputIndex,
		Delta:       `{"cmd":"ls"}`,
	})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:        responsesEventOutputItemAdded,
		OutputIndex: &outputIndex,
		Item: &dto.ResponsesOutput{
			Type:   responsesOutputTypeFunctionCall,
			ID:     "fc_1",
			CallId: "call_1",
			Name:   "exec",
		},
	})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type: responsesEventCompleted,
		Response: &dto.OpenAIResponsesResponse{
			Status: []byte(`"completed"`),
			Usage:  &dto.Usage{InputTokens: 1, OutputTokens: 2, TotalTokens: 3},
		},
	})...)

	require.Len(t, chunks, 4)
	assert.Equal(t, "assistant", chunks[0].Choices[0].Delta.Role)
	assert.Equal(t, "text before tool", chunks[1].Choices[0].Delta.GetContentString())
	tool := chunks[2].Choices[0].Delta.ToolCalls[0]
	require.NotNil(t, tool.Index)
	assert.Equal(t, 0, *tool.Index)
	assert.Equal(t, "call_1", tool.ID)
	assert.Equal(t, "exec", tool.Function.Name)
	assert.Equal(t, `{"cmd":"ls"}`, tool.Function.Arguments)
	require.NotNil(t, chunks[3].Choices[0].FinishReason)
	assert.Equal(t, "tool_calls", *chunks[3].Choices[0].FinishReason)
	assert.Equal(t, 3, state.Usage.TotalTokens)
}

func TestResponsesStreamEventToChatChunksDoesNotDuplicatePendingArgsWithOutputIndexAndItemID(t *testing.T) {
	state := newTestResponsesStreamState()
	outputIndex := 1

	var chunks []dto.ChatCompletionsStreamResponse
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{Type: responsesEventCreated})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:        responsesEventFunctionArgsDelta,
		OutputIndex: &outputIndex,
		ItemID:      "fc_1",
		Delta:       `{"q":"x"}`,
	})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:        responsesEventOutputItemAdded,
		OutputIndex: &outputIndex,
		ItemID:      "fc_1",
		Item: &dto.ResponsesOutput{
			Type:   responsesOutputTypeFunctionCall,
			ID:     "fc_1",
			CallId: "call_1",
			Name:   "lookup",
		},
	})...)

	require.Len(t, chunks, 2)
	tool := chunks[1].Choices[0].Delta.ToolCalls[0]
	assert.Equal(t, "call_1", tool.ID)
	assert.Equal(t, "lookup", tool.Function.Name)
	assert.Equal(t, `{"q":"x"}`, tool.Function.Arguments)
	assert.Empty(t, state.pendingArgsByOutputIndex)
	assert.Empty(t, state.pendingArgsByItemID)
}

func TestResponsesStreamEventToChatChunksDrainsItemOnlyPendingArgsWhenOutputIndexArrives(t *testing.T) {
	state := newTestResponsesStreamState()
	outputIndex := 1

	var chunks []dto.ChatCompletionsStreamResponse
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{Type: responsesEventCreated})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:   responsesEventFunctionArgsDelta,
		ItemID: "fc_1",
		Delta:  `{"q":"x"}`,
	})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:        responsesEventOutputItemAdded,
		OutputIndex: &outputIndex,
		ItemID:      "fc_1",
		Item: &dto.ResponsesOutput{
			Type:   responsesOutputTypeFunctionCall,
			CallId: "call_1",
			Name:   "lookup",
		},
	})...)

	require.Len(t, chunks, 2)
	tool := chunks[1].Choices[0].Delta.ToolCalls[0]
	assert.Equal(t, "call_1", tool.ID)
	assert.Equal(t, "lookup", tool.Function.Name)
	assert.Equal(t, `{"q":"x"}`, tool.Function.Arguments)
	assert.Empty(t, state.pendingArgsByOutputIndex)
	assert.Empty(t, state.pendingArgsByItemID)
}

func TestResponsesStreamEventToChatChunksCustomToolAndReasoning(t *testing.T) {
	state := newTestResponsesStreamState()
	outputIndex := 0

	chunks := mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:  responsesEventReasoningTextDelta,
		Delta: "thinking",
	})
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:        responsesEventOutputItemAdded,
		OutputIndex: &outputIndex,
		Item: &dto.ResponsesOutput{
			Type:   responsesOutputTypeCustomToolCall,
			ID:     "ct_1",
			CallId: "call_custom",
			Name:   "apply_patch",
		},
	})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:        responsesEventCustomToolInputDelta,
		OutputIndex: &outputIndex,
		Delta:       "patch body",
	})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type: responsesEventIncomplete,
		Response: &dto.OpenAIResponsesResponse{
			IncompleteDetails: &dto.IncompleteDetails{Reason: responsesIncompleteReasonContentFilter},
		},
	})...)

	require.Len(t, chunks, 5)
	assert.Equal(t, "thinking", chunks[1].Choices[0].Delta.GetReasoningContent())
	assert.Equal(t, "apply_patch", chunks[2].Choices[0].Delta.ToolCalls[0].Function.Name)
	assert.Equal(t, "patch body", chunks[3].Choices[0].Delta.ToolCalls[0].Function.Arguments)
	require.NotNil(t, chunks[4].Choices[0].FinishReason)
	assert.Equal(t, "content_filter", *chunks[4].Choices[0].FinishReason)
}

func TestResponsesStreamEventToChatChunksWrapsNamespacedToolCalls(t *testing.T) {
	state := newTestResponsesStreamState()
	outputIndex := 0

	chunks := mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:        responsesEventOutputItemAdded,
		OutputIndex: &outputIndex,
		Item: &dto.ResponsesOutput{
			Type:      responsesOutputTypeFunctionCall,
			ID:        "fc_1",
			CallId:    "call_1",
			Name:      "spawn_agent",
			Namespace: "multi_agent_v1",
		},
	})
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:        responsesEventFunctionArgsDelta,
		OutputIndex: &outputIndex,
		Delta:       `{"agent_type":"explore"}`,
	})...)

	require.Len(t, chunks, 3)
	toolName := chunks[1].Choices[0].Delta.ToolCalls[0].Function.Name
	assert.Equal(t, "codex__multi_agent_v1__spawn_agent", toolName)
	toolArgs := chunks[2].Choices[0].Delta.ToolCalls[0].Function.Arguments
	assert.Equal(t, `{"agent_type":"explore"}`, toolArgs)
}

func TestResponsesStreamEventToChatChunksUsesTerminalDoneOutput(t *testing.T) {
	state := newTestResponsesStreamState()
	chunks := mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type: responsesEventDone,
		Response: &dto.OpenAIResponsesResponse{
			Status: []byte(`"completed"`),
			Output: []dto.ResponsesOutput{
				{
					Type: responsesOutputTypeMessage,
					Role: "assistant",
					Content: []dto.ResponsesOutputContent{
						{Type: "output_text", Text: "terminal text"},
					},
				},
				{
					Type:      responsesOutputTypeFunctionCall,
					ID:        "fc_1",
					CallId:    "call_1",
					Name:      "lookup",
					Arguments: []byte(`{"q":"x"}`),
				},
			},
		},
	})

	require.Len(t, chunks, 4)
	assert.Equal(t, "assistant", chunks[0].Choices[0].Delta.Role)
	assert.Equal(t, "terminal text", chunks[1].Choices[0].Delta.GetContentString())
	tool := chunks[2].Choices[0].Delta.ToolCalls[0]
	assert.Equal(t, "lookup", tool.Function.Name)
	assert.Equal(t, `{"q":"x"}`, tool.Function.Arguments)
	require.NotNil(t, chunks[3].Choices[0].FinishReason)
	assert.Equal(t, "tool_calls", *chunks[3].Choices[0].FinishReason)
}

func TestResponsesStreamEventToChatChunksRecoversTextFromDoneItemWhenCompletedOutputIsEmpty(t *testing.T) {
	state := newTestResponsesStreamState()

	chunks := mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type: responsesEventOutputItemDone,
		Item: &dto.ResponsesOutput{
			Type: responsesOutputTypeMessage,
			Role: "assistant",
			Content: []dto.ResponsesOutputContent{
				{Type: "output_text", Text: "scan project login pages"},
			},
		},
	})
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type: responsesEventCompleted,
		Response: &dto.OpenAIResponsesResponse{
			Status: []byte(`"completed"`),
			Output: []dto.ResponsesOutput{},
			Usage:  &dto.Usage{InputTokens: 386, OutputTokens: 15, TotalTokens: 401},
		},
	})...)

	require.Len(t, chunks, 3)
	assert.Equal(t, "assistant", chunks[0].Choices[0].Delta.Role)
	assert.Equal(t, "scan project login pages", chunks[1].Choices[0].Delta.GetContentString())
	require.NotNil(t, chunks[2].Choices[0].FinishReason)
	assert.Equal(t, "stop", *chunks[2].Choices[0].FinishReason)
	assert.Equal(t, 401, state.Usage.TotalTokens)
}

func TestResponsesStreamEventToChatChunksDoneEventsOnlyAppendMissingText(t *testing.T) {
	state := newTestResponsesStreamState()

	chunks := mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type:  responsesEventOutputTextDelta,
		Delta: "scan project",
	})
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type: responsesEventOutputTextDone,
		Text: "scan project login pages",
	})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type: responsesEventContentPartDone,
		Part: &dto.ResponsesReasoningSummaryPart{
			Type: "output_text",
			Text: "scan project login pages",
		},
	})...)
	chunks = append(chunks, mustStreamChunks(t, state, &dto.ResponsesStreamResponse{
		Type: responsesEventOutputItemDone,
		Item: &dto.ResponsesOutput{
			Type: responsesOutputTypeMessage,
			Role: "assistant",
			Content: []dto.ResponsesOutputContent{
				{Type: "output_text", Text: "scan project login pages"},
			},
		},
	})...)

	require.Len(t, chunks, 3)
	assert.Equal(t, "assistant", chunks[0].Choices[0].Delta.Role)
	assert.Equal(t, "scan project", chunks[1].Choices[0].Delta.GetContentString())
	assert.Equal(t, " login pages", chunks[2].Choices[0].Delta.GetContentString())
}

func TestFinalizeResponsesToChatStreamFlushesPendingDeltaOnlyArguments(t *testing.T) {
	state := newTestResponsesStreamState()
	outputIndex := 2
	_, err := ResponsesStreamEventToChatChunks(&dto.ResponsesStreamResponse{
		Type:        responsesEventFunctionArgsDelta,
		OutputIndex: &outputIndex,
		Delta:       `{"pending":true}`,
	}, state)
	require.NoError(t, err)

	chunks := FinalizeResponsesToChatStream(state)
	require.Len(t, chunks, 3)
	tool := chunks[1].Choices[0].Delta.ToolCalls[0]
	assert.Equal(t, "call_output_2", tool.ID)
	assert.Equal(t, `{"pending":true}`, tool.Function.Arguments)
	require.NotNil(t, chunks[2].Choices[0].FinishReason)
	assert.Equal(t, "tool_calls", *chunks[2].Choices[0].FinishReason)
}

func TestResponsesStreamEventToChatChunksFailedEventReturnsError(t *testing.T) {
	_, err := ResponsesStreamEventToChatChunks(&dto.ResponsesStreamResponse{Type: responsesEventFailed}, newTestResponsesStreamState())
	require.Error(t, err)
}

func TestResponsesBufferedAccumulatorSupplementsEmptyTerminalOutput(t *testing.T) {
	acc := NewResponsesBufferedAccumulator()
	outputIndex := 1
	acc.ProcessEvent(&dto.ResponsesStreamResponse{Type: responsesEventOutputTextDelta, Delta: "buffered text"})
	acc.ProcessEvent(&dto.ResponsesStreamResponse{
		Type:        responsesEventOutputItemAdded,
		OutputIndex: &outputIndex,
		Item: &dto.ResponsesOutput{
			Type:   responsesOutputTypeFunctionCall,
			ID:     "fc_1",
			CallId: "call_1",
			Name:   "lookup",
		},
	})
	acc.ProcessEvent(&dto.ResponsesStreamResponse{
		Type:        responsesEventFunctionArgsDelta,
		OutputIndex: &outputIndex,
		Delta:       `{"q":"x"}`,
	})

	resp := &dto.OpenAIResponsesResponse{
		Status: []byte(`"completed"`),
		Model:  "gpt-test",
	}
	acc.SupplementResponseOutput(resp)

	chat, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_1")
	require.NoError(t, err)
	assert.Equal(t, "buffered text", chat.Choices[0].Message.StringContent())
	toolCalls := chat.Choices[0].Message.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, `{"q":"x"}`, toolCalls[0].Function.Arguments)
}

func TestResponsesBufferedAccumulatorRecoversDoneOnlyTextWithoutDuplication(t *testing.T) {
	acc := NewResponsesBufferedAccumulator()
	acc.ProcessEvent(&dto.ResponsesStreamResponse{
		Type:  responsesEventOutputTextDelta,
		Delta: "scan project",
	})
	acc.ProcessEvent(&dto.ResponsesStreamResponse{
		Type: responsesEventOutputTextDone,
		Text: "scan project login pages",
	})
	acc.ProcessEvent(&dto.ResponsesStreamResponse{
		Type: responsesEventOutputItemDone,
		Item: &dto.ResponsesOutput{
			Type: responsesOutputTypeMessage,
			Role: "assistant",
			Content: []dto.ResponsesOutputContent{
				{Type: "output_text", Text: "scan project login pages"},
			},
		},
	})

	resp := &dto.OpenAIResponsesResponse{
		Status: []byte(`"completed"`),
		Usage:  &dto.Usage{InputTokens: 386, OutputTokens: 15, TotalTokens: 401},
	}
	acc.SupplementResponseOutput(resp)

	chat, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_1")
	require.NoError(t, err)
	assert.Equal(t, "scan project login pages", chat.Choices[0].Message.StringContent())
}

func TestResponsesBufferedAccumulatorPreservesToolNamespace(t *testing.T) {
	acc := NewResponsesBufferedAccumulator()
	outputIndex := 1
	acc.ProcessEvent(&dto.ResponsesStreamResponse{
		Type:        responsesEventOutputItemAdded,
		OutputIndex: &outputIndex,
		Item: &dto.ResponsesOutput{
			Type:      responsesOutputTypeFunctionCall,
			ID:        "fc_1",
			CallId:    "call_1",
			Name:      "spawn_agent",
			Namespace: "multi_agent_v1",
		},
	})
	acc.ProcessEvent(&dto.ResponsesStreamResponse{
		Type:        responsesEventFunctionArgsDelta,
		OutputIndex: &outputIndex,
		Delta:       `{"agent_type":"explore"}`,
	})

	resp := &dto.OpenAIResponsesResponse{Status: []byte(`"completed"`)}
	acc.SupplementResponseOutput(resp)

	chat, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_1")
	require.NoError(t, err)
	toolCalls := chat.Choices[0].Message.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "codex__multi_agent_v1__spawn_agent", toolCalls[0].Function.Name)
}

func TestResponsesBufferedAccumulatorDoesNotDuplicatePendingArgsWithOutputIndexAndItemID(t *testing.T) {
	acc := NewResponsesBufferedAccumulator()
	outputIndex := 1
	acc.ProcessEvent(&dto.ResponsesStreamResponse{
		Type:        responsesEventFunctionArgsDelta,
		OutputIndex: &outputIndex,
		ItemID:      "fc_1",
		Delta:       `{"q":"x"}`,
	})
	acc.ProcessEvent(&dto.ResponsesStreamResponse{
		Type:        responsesEventOutputItemAdded,
		OutputIndex: &outputIndex,
		ItemID:      "fc_1",
		Item: &dto.ResponsesOutput{
			Type:   responsesOutputTypeFunctionCall,
			ID:     "fc_1",
			CallId: "call_1",
			Name:   "lookup",
		},
	})

	resp := &dto.OpenAIResponsesResponse{
		Status: []byte(`"completed"`),
		Model:  "gpt-test",
	}
	acc.SupplementResponseOutput(resp)

	chat, _, err := ResponsesResponseToChatCompletionsResponse(resp, "chatcmpl_1")
	require.NoError(t, err)
	toolCalls := chat.Choices[0].Message.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, `{"q":"x"}`, toolCalls[0].Function.Arguments)
	assert.Empty(t, acc.pendingByOutputIndex)
	assert.Empty(t, acc.pendingByItemID)
}

func TestChatCompletionsResponseToResponsesPreservesTextToolCallsAndUsage(t *testing.T) {
	chat := &dto.OpenAITextResponse{
		Id:      "chatcmpl_1",
		Model:   "gpt-test",
		Created: 456,
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message:      assistantMessageWithTool("I will call.", "call_1", "lookup", `{"q":"x"}`),
				FinishReason: "tool_calls",
			},
		},
		Usage: dto.Usage{PromptTokens: 3, CompletionTokens: 5, TotalTokens: 8},
	}

	resp, usage, err := ChatCompletionsResponseToResponsesResponse(chat, "resp_1")
	require.NoError(t, err)
	require.NotNil(t, usage)

	assert.Equal(t, "resp_1", resp.ID)
	assert.Equal(t, "response", resp.Object)
	assert.Equal(t, `"completed"`, string(resp.Status))
	assert.Equal(t, 3, resp.Usage.InputTokens)
	assert.Equal(t, 5, resp.Usage.OutputTokens)
	require.Len(t, resp.Output, 2)
	assert.Equal(t, responsesOutputTypeMessage, resp.Output[0].Type)
	assert.Equal(t, "I will call.", resp.Output[0].Content[0].Text)
	assert.Equal(t, responsesOutputTypeFunctionCall, resp.Output[1].Type)
	assert.Equal(t, "call_1", resp.Output[1].CallId)
	assert.Equal(t, "lookup", resp.Output[1].Name)
	assert.Equal(t, `"{\"q\":\"x\"}"`, string(resp.Output[1].Arguments))
}

func TestChatCompletionsResponseToResponsesUnwrapsCodexCompatToolCalls(t *testing.T) {
	chat := &dto.OpenAITextResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message:      assistantMessageWithTool("", "call_1", "codex__namespace__exec_command", `{"cmd":"pwd"}`),
				FinishReason: "tool_calls",
			},
		},
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(chat, "resp_1")
	require.NoError(t, err)

	require.Len(t, resp.Output, 1)
	assert.Equal(t, responsesOutputTypeFunctionCall, resp.Output[0].Type)
	assert.Equal(t, "exec_command", resp.Output[0].Name)
	assert.Equal(t, "namespace", resp.Output[0].Namespace)
	assert.Equal(t, `"{\"cmd\":\"pwd\"}"`, string(resp.Output[0].Arguments))
}

func TestChatCompletionsResponseToResponsesRestoresMultiAgentNamespace(t *testing.T) {
	chat := &dto.OpenAITextResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message:      assistantMessageWithTool("", "call_1", "codex__multi_agent_v1__spawn_agent", `{"agent_type":"explore"}`),
				FinishReason: "tool_calls",
			},
		},
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(chat, "resp_1")
	require.NoError(t, err)

	require.Len(t, resp.Output, 1)
	assert.Equal(t, responsesOutputTypeFunctionCall, resp.Output[0].Type)
	assert.Equal(t, "spawn_agent", resp.Output[0].Name)
	assert.Equal(t, "multi_agent_v1", resp.Output[0].Namespace)
	assert.Equal(t, `"{\"agent_type\":\"explore\"}"`, string(resp.Output[0].Arguments))
}

func TestChatCompletionsResponseToResponsesRestoresCustomToolCall(t *testing.T) {
	chat := &dto.OpenAITextResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.OpenAITextResponseChoice{
			{
				Message:      assistantMessageWithTool("", "call_1", "codex__custom__apply_patch", `{"input":"patch body"}`),
				FinishReason: "tool_calls",
			},
		},
	}

	resp, _, err := ChatCompletionsResponseToResponsesResponse(chat, "resp_1")
	require.NoError(t, err)

	require.Len(t, resp.Output, 1)
	assert.Equal(t, responsesOutputTypeCustomToolCall, resp.Output[0].Type)
	assert.Equal(t, "apply_patch", resp.Output[0].Name)
	assert.Equal(t, "patch body", resp.Output[0].Input)
	assert.Empty(t, resp.Output[0].Arguments)
}

func TestChatCompletionsResponseToResponsesMapsIncompleteFinishReasons(t *testing.T) {
	tests := []struct {
		name         string
		finishReason string
		wantReason   string
	}{
		{name: "length", finishReason: "length", wantReason: responsesIncompleteReasonMaxTokens},
		{name: "content filter", finishReason: "content_filter", wantReason: responsesIncompleteReasonContentFilter},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _, err := ChatCompletionsResponseToResponsesResponse(&dto.OpenAITextResponse{
				Id:    "chatcmpl_1",
				Model: "gpt-test",
				Choices: []dto.OpenAITextResponseChoice{
					{
						Message:      dto.Message{Role: "assistant", Content: "partial"},
						FinishReason: tt.finishReason,
					},
				},
			}, "resp_1")
			require.NoError(t, err)

			assert.Equal(t, `"incomplete"`, string(resp.Status))
			require.NotNil(t, resp.IncompleteDetails)
			assert.Equal(t, tt.wantReason, resp.IncompleteDetails.Reason)
			require.Len(t, resp.Output, 1)
			assert.Equal(t, "incomplete", resp.Output[0].Status)
		})
	}
}

func TestChatCompletionsStreamToResponsesEventsAggregatesUsageAndToolArgs(t *testing.T) {
	state := NewChatToResponsesStreamState("resp_1", "gpt-test")
	state.Created = 123
	toolIndex := 0

	var events []ChatToResponsesStreamEvent
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Id:      "chatcmpl_1",
		Model:   "gpt-test",
		Created: 123,
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Role: "assistant"}},
		},
	})...)
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{Content: lo.ToPtr("hello")}},
		},
	})...)
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{
				{Index: &toolIndex, ID: "call_1", Type: "function", Function: dto.FunctionResponse{Name: "lookup"}},
			}}},
		},
	})...)
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{
				{Index: &toolIndex, Function: dto.FunctionResponse{Arguments: `{"q":"x"}`}},
			}}},
		},
	})...)
	finishReason := "tool_calls"
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, FinishReason: &finishReason},
		},
	})...)
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Usage: &dto.Usage{PromptTokens: 2, CompletionTokens: 4, TotalTokens: 6},
	})...)
	events = append(events, FinalizeChatCompletionsStreamToResponses(state)...)

	require.Len(t, events, 10)
	assert.Equal(t, responsesEventCreated, events[0].Type)
	assert.Equal(t, responsesEventOutputTextDelta, events[2].Type)
	assert.Equal(t, "hello", events[2].Payload.Delta)
	assert.Equal(t, responsesEventFunctionArgsDelta, events[4].Type)
	assert.Equal(t, `{"q":"x"}`, events[4].Payload.Delta)
	assert.Equal(t, responsesEventCompleted, events[9].Type)
	require.NotNil(t, events[9].Payload.Response)
	assert.Equal(t, 6, events[9].Payload.Response.Usage.TotalTokens)
	require.Len(t, events[9].Payload.Response.Output, 2)
	assert.Equal(t, "hello", events[9].Payload.Response.Output[0].Content[0].Text)
	assert.Equal(t, `"{\"q\":\"x\"}"`, string(events[9].Payload.Response.Output[1].Arguments))
}

func TestChatCompletionsStreamToResponsesReasoningSummaryOrder(t *testing.T) {
	state := NewChatToResponsesStreamState("resp_1", "gpt-test")
	reasoning := "think"
	finishReason := "stop"

	var events []ChatToResponsesStreamEvent
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{
				Index: 0,
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					ReasoningContent: &reasoning,
				},
			},
		},
	})...)
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, FinishReason: &finishReason},
		},
	})...)
	events = append(events, FinalizeChatCompletionsStreamToResponses(state)...)

	require.Len(t, events, 8)
	assert.Equal(t, responsesEventCreated, events[0].Type)
	assert.Equal(t, responsesEventOutputItemAdded, events[1].Type)
	assert.Equal(t, responsesEventReasoningSummaryPartAdded, events[2].Type)
	assert.Equal(t, responsesEventReasoningSummaryDelta, events[3].Type)
	assert.Equal(t, responsesEventReasoningSummaryDone, events[4].Type)
	assert.Equal(t, responsesEventReasoningSummaryPartDone, events[5].Type)
	assert.Equal(t, responsesEventOutputItemDone, events[6].Type)
	assert.Equal(t, responsesEventCompleted, events[7].Type)

	assert.Equal(t, reasoning, events[3].Payload.Delta)
	assert.Equal(t, reasoning, events[4].Payload.Text)
	require.NotNil(t, events[5].Payload.Part)
	assert.Equal(t, reasoning, events[5].Payload.Part.Text)
	assert.Equal(t, 0, *events[2].Payload.SummaryIndex)
	assert.Equal(t, 0, *events[3].Payload.SummaryIndex)
}

func TestChatCompletionsStreamToResponsesRestoresCustomToolInputEvents(t *testing.T) {
	state := NewChatToResponsesStreamState("resp_1", "gpt-test")
	toolIndex := 0

	var events []ChatToResponsesStreamEvent
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{
				{Index: &toolIndex, ID: "call_1", Type: "function", Function: dto.FunctionResponse{Name: "codex__custom__apply_patch"}},
			}}},
		},
	})...)
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{
				{Index: &toolIndex, Function: dto.FunctionResponse{Arguments: `{"input":"patch body"}`}},
			}}},
		},
	})...)
	finishReason := "tool_calls"
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, FinishReason: &finishReason},
		},
	})...)
	events = append(events, FinalizeChatCompletionsStreamToResponses(state)...)

	require.Len(t, events, 6)
	assert.Equal(t, responsesEventOutputItemAdded, events[1].Type)
	assert.Equal(t, responsesOutputTypeCustomToolCall, events[1].Payload.Item.Type)
	assert.Empty(t, events[1].Payload.Item.Namespace)
	assert.Equal(t, responsesEventCustomToolInputDelta, events[2].Type)
	assert.Equal(t, "patch body", events[2].Payload.Delta)
	assert.Equal(t, responsesEventCustomToolInputDone, events[3].Type)
	assert.Equal(t, "patch body", events[3].Payload.Input)
	assert.Equal(t, responsesEventOutputItemDone, events[4].Type)
	assert.Equal(t, responsesOutputTypeCustomToolCall, events[4].Payload.Item.Type)
	assert.Equal(t, "patch body", events[4].Payload.Item.Input)
	assert.Equal(t, responsesEventCompleted, events[5].Type)
}

func TestChatCompletionsRequestToResponsesRequestAssistantNilContentWithToolCallsOmitsEmptyAssistantItem(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-test",
		N:     lo.ToPtr(1),
		Messages: []dto.Message{
			{Role: "user", Content: "call a tool"},
			{Role: "assistant", Content: nil, ToolCalls: mustMarshalToolCallsJSON(t, []dto.ToolCallRequest{
				{ID: "call_1", Type: "function", Function: dto.FunctionRequest{Name: "lookup", Arguments: `{"q":"x"}`}},
			})},
			{Role: "tool", ToolCallId: "call_1", Content: "result"},
		},
	}

	got, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	require.Equal(t, int64(3), gjson.GetBytes(got.Input, "#").Int())
	require.False(t, gjson.GetBytes(got.Input, "1.role").Exists(), "no empty assistant role item should exist")
	require.Equal(t, "function_call", gjson.GetBytes(got.Input, "1.type").String())
	assert.Equal(t, "call_1", gjson.GetBytes(got.Input, "1.call_id").String())
	assert.Equal(t, "function_call_output", gjson.GetBytes(got.Input, "2.type").String())
}

func TestChatCompletionsRequestToResponsesRequestAssistantEmptyContentPartsWithToolCallsSkipsEmptyItem(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-test",
		N:     lo.ToPtr(1),
		Messages: []dto.Message{
			{Role: "user", Content: "call a tool"},
			{Role: "assistant", Content: []any{}, ToolCalls: mustMarshalToolCallsJSON(t, []dto.ToolCallRequest{
				{ID: "call_1", Type: "function", Function: dto.FunctionRequest{Name: "lookup", Arguments: `{"q":"x"}`}},
			})},
			{Role: "tool", ToolCallId: "call_1", Content: "result"},
		},
	}

	got, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	require.Equal(t, int64(3), gjson.GetBytes(got.Input, "#").Int())
	require.False(t, gjson.GetBytes(got.Input, "1.role").Exists(), "no empty assistant role item should exist")
	require.Equal(t, "function_call", gjson.GetBytes(got.Input, "1.type").String())
	assert.Equal(t, "call_1", gjson.GetBytes(got.Input, "1.call_id").String())
	assert.Equal(t, "function_call_output", gjson.GetBytes(got.Input, "2.type").String())
}

func TestChatCompletionsRequestToResponsesRequestAssistantEmptyStringContentWithToolCallsSkipsEmptyItem(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-test",
		N:     lo.ToPtr(1),
		Messages: []dto.Message{
			{Role: "user", Content: "call a tool"},
			{Role: "assistant", Content: "", ToolCalls: mustMarshalToolCallsJSON(t, []dto.ToolCallRequest{
				{ID: "call_1", Type: "function", Function: dto.FunctionRequest{Name: "lookup", Arguments: `{"q":"x"}`}},
			})},
			{Role: "tool", ToolCallId: "call_1", Content: "result"},
		},
	}

	got, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	require.Equal(t, int64(3), gjson.GetBytes(got.Input, "#").Int())
	require.False(t, gjson.GetBytes(got.Input, "1.role").Exists(), "no assistant role item should exist")
	require.Equal(t, "function_call", gjson.GetBytes(got.Input, "1.type").String())
	assert.Equal(t, "function_call_output", gjson.GetBytes(got.Input, "2.type").String())
}

func TestChatCompletionsRequestToResponsesRequestAssistantNonEmptyContentWithToolCallsPreserved(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-test",
		N:     lo.ToPtr(1),
		Messages: []dto.Message{
			{Role: "user", Content: "call a tool"},
			assistantMessageWithTool("I'll look that up", "call_1", "lookup", `{"q":"x"}`),
			{Role: "tool", ToolCallId: "call_1", Content: "result"},
		},
	}

	got, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	require.Equal(t, int64(4), gjson.GetBytes(got.Input, "#").Int())
	require.Equal(t, "assistant", gjson.GetBytes(got.Input, "1.role").String())
	require.Equal(t, "I'll look that up", gjson.GetBytes(got.Input, "1.content").String())
	require.Equal(t, "function_call", gjson.GetBytes(got.Input, "2.type").String())
	assert.Equal(t, "function_call_output", gjson.GetBytes(got.Input, "3.type").String())
}

func TestChatCompletionsRequestToResponsesRequestAssistantNilContentWithoutToolCallsEmitsEmptyItem(t *testing.T) {
	req := &dto.GeneralOpenAIRequest{
		Model: "gpt-test",
		N:     lo.ToPtr(1),
		Messages: []dto.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: nil},
			{Role: "user", Content: "follow up"},
		},
	}

	got, err := ChatCompletionsRequestToResponsesRequest(req)
	require.NoError(t, err)

	require.Equal(t, int64(3), gjson.GetBytes(got.Input, "#").Int())
	require.Equal(t, "assistant", gjson.GetBytes(got.Input, "1.role").String())
	assert.Equal(t, "", gjson.GetBytes(got.Input, "1.content").String())
}

func mustMarshalToolCallsJSON(t *testing.T, toolCalls []dto.ToolCallRequest) json.RawMessage {
	t.Helper()
	b, err := common.Marshal(toolCalls)
	require.NoError(t, err)
	return b
}

func TestChatCompletionsStreamToResponsesRestoresMultiAgentNamespace(t *testing.T) {
	state := NewChatToResponsesStreamState("resp_1", "gpt-test")
	toolIndex := 0

	var events []ChatToResponsesStreamEvent
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Id:    "chatcmpl_1",
		Model: "gpt-test",
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{
				{Index: &toolIndex, ID: "call_1", Type: "function", Function: dto.FunctionResponse{Name: "codex__multi_agent_v1__spawn_agent"}},
			}}},
		},
	})...)
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, Delta: dto.ChatCompletionsStreamResponseChoiceDelta{ToolCalls: []dto.ToolCallResponse{
				{Index: &toolIndex, Function: dto.FunctionResponse{Arguments: `{"agent_type":"explore"}`}},
			}}},
		},
	})...)
	finishReason := "tool_calls"
	events = append(events, mustResponsesEventsFromChatChunk(t, state, &dto.ChatCompletionsStreamResponse{
		Choices: []dto.ChatCompletionsStreamResponseChoice{
			{Index: 0, FinishReason: &finishReason},
		},
	})...)
	events = append(events, FinalizeChatCompletionsStreamToResponses(state)...)

	require.Len(t, events, 6)
	assert.Equal(t, responsesEventOutputItemAdded, events[1].Type)
	assert.Equal(t, "spawn_agent", events[1].Payload.Item.Name)
	assert.Equal(t, "multi_agent_v1", events[1].Payload.Item.Namespace)
	assert.Equal(t, responsesEventFunctionArgsDone, events[3].Type)
	assert.Equal(t, `{"agent_type":"explore"}`, events[3].Payload.Arguments)
	assert.Equal(t, responsesEventOutputItemDone, events[4].Type)
	assert.Equal(t, "multi_agent_v1", events[4].Payload.Item.Namespace)
	assert.Equal(t, responsesEventCompleted, events[5].Type)
}

func assistantMessageWithTool(content string, id string, name string, args string) dto.Message {
	msg := dto.Message{Role: "assistant", Content: content}
	msg.SetToolCalls([]dto.ToolCallRequest{
		{
			ID:   id,
			Type: "function",
			Function: dto.FunctionRequest{
				Name:      name,
				Arguments: args,
			},
		},
	})
	return msg
}

func newTestResponsesStreamState() *ResponsesToChatStreamState {
	state := NewResponsesToChatStreamState("gpt-test", false)
	state.ID = "chatcmpl_test"
	state.Created = 123
	return state
}

func mustStreamChunks(t *testing.T, state *ResponsesToChatStreamState, event *dto.ResponsesStreamResponse) []dto.ChatCompletionsStreamResponse {
	t.Helper()
	chunks, err := ResponsesStreamEventToChatChunks(event, state)
	require.NoError(t, err)
	return chunks
}

func mustResponsesEventsFromChatChunk(t *testing.T, state *ChatToResponsesStreamState, chunk *dto.ChatCompletionsStreamResponse) []ChatToResponsesStreamEvent {
	t.Helper()
	events, err := ChatCompletionsStreamChunkToResponsesEvents(chunk, state)
	require.NoError(t, err)
	return events
}
