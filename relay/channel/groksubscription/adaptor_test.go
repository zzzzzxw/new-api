package groksubscription

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSetupRequestHeaderUsesOAuthAccessTokenAndCLIIdentity(t *testing.T) {
	credential, err := common.Marshal(OAuthKey{AccessToken: "access-secret"})
	require.NoError(t, err)

	headers := http.Header{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: string(credential),
		},
	}
	context := &gin.Context{Request: httptest.NewRequest(http.MethodPost, "/v1/responses", nil)}
	err = (&Adaptor{}).SetupRequestHeader(context, &headers, info)
	require.NoError(t, err)
	require.Equal(t, "Bearer access-secret", headers.Get("Authorization"))
	require.Equal(t, DefaultGrokCLIVersion, headers.Get("X-Grok-Client-Version"))
	require.Equal(t, "interactive", headers.Get("X-Grok-Client-Mode"))
}

func TestSetupRequestHeaderAllowsCLIVersionOverride(t *testing.T) {
	t.Setenv("XAI_GROK_CLI_VERSION", "9.9.9")
	credential, err := common.Marshal(OAuthKey{AccessToken: "access-secret"})
	require.NoError(t, err)

	headers := http.Header{}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: string(credential),
		},
	}
	context := &gin.Context{Request: httptest.NewRequest(http.MethodPost, "/v1/responses", nil)}
	require.NoError(t, (&Adaptor{}).SetupRequestHeader(context, &headers, info))
	require.Equal(t, "9.9.9", headers.Get("X-Grok-Client-Version"))
}

func TestConvertResponsesPromotesAdditionalToolsAndDropsUnsupportedTools(t *testing.T) {
	input, err := common.Marshal([]map[string]any{
		{"type": "message", "role": "user", "content": "hello"},
		{
			"type": "additional_tools",
			"tools": []any{
				map[string]any{"type": "shell", "name": "shell"},
				map[string]any{"type": "computer", "name": "computer"},
			},
		},
	})
	require.NoError(t, err)

	request := dto.OpenAIResponsesRequest{Model: "grok-4.5", Input: input}
	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, nil, request)
	require.NoError(t, err)
	normalized := converted.(dto.OpenAIResponsesRequest)

	var normalizedInput []map[string]any
	require.NoError(t, common.Unmarshal(normalized.Input, &normalizedInput))
	require.Len(t, normalizedInput, 1)

	var normalizedTools []map[string]any
	require.NoError(t, common.Unmarshal(normalized.Tools, &normalizedTools))
	require.Len(t, normalizedTools, 1)
	require.Equal(t, "shell", normalizedTools[0]["type"])
}

func TestConvertResponsesLowersCustomToolsAndHistory(t *testing.T) {
	input, err := common.Marshal([]map[string]any{
		{"type": "custom_tool_call", "id": "item-1", "call_id": "call-1", "name": "apply_patch", "input": "*** Begin Patch"},
		{"type": "custom_tool_call_output", "call_id": "call-1", "output": map[string]any{"ok": true}},
	})
	require.NoError(t, err)
	tools, err := common.Marshal([]map[string]any{{
		"type": "custom", "name": "apply_patch", "description": "Apply a patch", "format": map[string]any{"type": "grammar"},
	}})
	require.NoError(t, err)
	toolChoice, err := common.Marshal(map[string]any{"type": "custom", "name": "apply_patch"})
	require.NoError(t, err)

	context := &gin.Context{}
	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(context, nil, dto.OpenAIResponsesRequest{
		Model: "grok-4.5", Input: input, Tools: tools, ToolChoice: toolChoice,
	})
	require.NoError(t, err)
	normalized := converted.(dto.OpenAIResponsesRequest)

	var normalizedTools []map[string]any
	require.NoError(t, common.Unmarshal(normalized.Tools, &normalizedTools))
	require.Equal(t, "function", normalizedTools[0]["type"])
	require.NotNil(t, normalizedTools[0]["parameters"])
	require.NotContains(t, normalizedTools[0], "format")

	var normalizedInput []map[string]any
	require.NoError(t, common.Unmarshal(normalized.Input, &normalizedInput))
	require.Equal(t, "function_call", normalizedInput[0]["type"])
	require.JSONEq(t, `{"input":"*** Begin Patch"}`, normalizedInput[0]["arguments"].(string))
	require.Equal(t, "function_call_output", normalizedInput[1]["type"])
	require.JSONEq(t, `{"ok":true}`, normalizedInput[1]["output"].(string))

	var normalizedChoice map[string]any
	require.NoError(t, common.Unmarshal(normalized.ToolChoice, &normalizedChoice))
	require.Equal(t, "function", normalizedChoice["type"])
	require.Equal(t, map[string]bool{"apply_patch": true}, customToolsFromContext(context))
}

func TestConvertResponsesDropsToolChoiceWhenToolsArrayIsEmpty(t *testing.T) {
	tools, err := common.Marshal([]map[string]any{})
	require.NoError(t, err)
	toolChoice, err := common.Marshal("auto")
	require.NoError(t, err)

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, nil, dto.OpenAIResponsesRequest{
		Model:      "grok-4.5",
		Tools:      tools,
		ToolChoice: toolChoice,
	})
	require.NoError(t, err)
	normalized := converted.(dto.OpenAIResponsesRequest)

	require.Empty(t, normalized.Tools)
	require.Empty(t, normalized.ToolChoice)
}

func TestConvertResponsesDropsToolChoiceWhenToolsAreOmitted(t *testing.T) {
	toolChoice, err := common.Marshal("auto")
	require.NoError(t, err)

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, nil, dto.OpenAIResponsesRequest{
		Model:      "grok-4.5",
		ToolChoice: toolChoice,
	})
	require.NoError(t, err)
	normalized := converted.(dto.OpenAIResponsesRequest)

	require.Empty(t, normalized.Tools)
	require.Empty(t, normalized.ToolChoice)
}

func TestRestoreCustomToolPayload(t *testing.T) {
	payload := []byte(`{"id":"resp-1","output":[{"type":"function_call","id":"item-1","call_id":"call-1","name":"apply_patch","arguments":"{\"input\":\"*** Begin Patch\"}"}]}`)
	restored, err := restoreCustomToolPayload(payload, map[string]bool{"apply_patch": true})
	require.NoError(t, err)

	var response map[string]any
	require.NoError(t, common.Unmarshal(restored, &response))
	output := response["output"].([]any)[0].(map[string]any)
	require.Equal(t, "custom_tool_call", output["type"])
	require.Equal(t, "*** Begin Patch", output["input"])
	require.NotContains(t, output, "arguments")
}

func TestCustomToolStreamRestorerBuffersArguments(t *testing.T) {
	restorer := newCustomToolStreamRestorer(map[string]bool{"apply_patch": true})
	events := []string{
		`{"type":"response.output_item.added","sequence_number":4,"output_index":0,"item":{"type":"function_call","id":"item-1","call_id":"call-1","name":"apply_patch","arguments":""}}`,
		`{"type":"response.function_call_arguments.delta","sequence_number":5,"output_index":0,"item_id":"item-1","delta":"{\"input\":\"*** Begin"}`,
		`{"type":"response.function_call_arguments.delta","sequence_number":6,"output_index":0,"item_id":"item-1","delta":" Patch\"}"}`,
		`{"type":"response.function_call_arguments.done","sequence_number":7,"output_index":0,"item_id":"item-1","arguments":"{\"input\":\"*** Begin Patch\"}"}`,
		`{"type":"response.output_item.done","sequence_number":8,"output_index":0,"item":{"type":"function_call","id":"item-1","call_id":"call-1","name":"apply_patch","arguments":"{\"input\":\"*** Begin Patch\"}"}}`,
	}

	var restored [][]byte
	for _, event := range events {
		output, err := restorer.transform([]byte(event))
		require.NoError(t, err)
		restored = append(restored, output...)
	}
	require.Len(t, restored, 4)

	var added, delta, done, itemDone map[string]any
	require.NoError(t, common.Unmarshal(restored[0], &added))
	require.NoError(t, common.Unmarshal(restored[1], &delta))
	require.NoError(t, common.Unmarshal(restored[2], &done))
	require.NoError(t, common.Unmarshal(restored[3], &itemDone))
	require.Equal(t, "custom_tool_call", added["item"].(map[string]any)["type"])
	require.Equal(t, "response.custom_tool_call_input.delta", delta["type"])
	require.Equal(t, "*** Begin Patch", delta["delta"])
	require.Equal(t, "response.custom_tool_call_input.done", done["type"])
	require.Equal(t, "*** Begin Patch", done["input"])
	require.Equal(t, "custom_tool_call", itemDone["item"].(map[string]any)["type"])
	require.Equal(t, float64(4), added["sequence_number"])
	require.Equal(t, float64(7), itemDone["sequence_number"])
}

func TestGetRequestURLUsesSubscriptionResponsesEndpoint(t *testing.T) {
	url, err := (&Adaptor{}).GetRequestURL(&relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://cli-chat-proxy.grok.com",
		},
	})
	require.NoError(t, err)
	require.Equal(t, "https://cli-chat-proxy.grok.com/v1/responses", url)
}
