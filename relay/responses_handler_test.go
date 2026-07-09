package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestSanitizeConvertedResponsesChatRequestJSONNormalizesFinalChatTools(t *testing.T) {
	body := mustRelayRawMessage(t, map[string]any{
		"model": "glm-5.2",
		"messages": []map[string]any{
			{"role": "user", "content": "hi"},
		},
		"tools": []map[string]any{
			{"type": "tool_search"},
			{"type": "mcp", "name": "browser.click"},
		},
	})

	got, err := sanitizeConvertedResponsesChatRequestJSON(&dto.GeneralOpenAIRequest{}, body)
	require.NoError(t, err)

	assert.Equal(t, "function", gjson.GetBytes(got, "tools.0.type").String())
	assert.Equal(t, "function", gjson.GetBytes(got, "tools.1.type").String())
	assert.Equal(t, "tool_search", gjson.GetBytes(got, "tools.0.function.name").String())
	assert.Equal(t, "codex__mcp__browser_click", gjson.GetBytes(got, "tools.1.function.name").String())
}

func TestSanitizeConvertedResponsesChatRequestJSONKeepsNativeResponsesTools(t *testing.T) {
	body := mustRelayRawMessage(t, map[string]any{
		"model": "gpt-test",
		"input": "hi",
		"tools": []map[string]any{
			{"type": "tool_search"},
		},
	})

	got, err := sanitizeConvertedResponsesChatRequestJSON(&dto.OpenAIResponsesRequest{}, body)
	require.NoError(t, err)

	assert.JSONEq(t, string(body), string(got))
}

func TestSanitizeConvertedResponsesChatRequestJSONNormalizesGLMResponsesTools(t *testing.T) {
	body := mustRelayRawMessage(t, map[string]any{
		"model": "glm-5.2",
		"input": "hi",
		"tools": []map[string]any{
			{"type": "tool_search"},
			{"type": "mcp", "name": "browser.click"},
		},
	})

	got, err := sanitizeConvertedResponsesChatRequestJSON(&dto.OpenAIResponsesRequest{}, body)
	require.NoError(t, err)

	assert.Equal(t, "function", gjson.GetBytes(got, "tools.0.type").String())
	assert.Equal(t, "function", gjson.GetBytes(got, "tools.1.type").String())
	assert.Equal(t, "tool_search", gjson.GetBytes(got, "tools.0.name").String())
	assert.Equal(t, "codex__mcp__browser_click", gjson.GetBytes(got, "tools.1.name").String())
	assert.False(t, gjson.GetBytes(got, "tools.0.function").Exists())
	assert.False(t, gjson.GetBytes(got, "tools.1.function").Exists())
}

func mustRelayRawMessage(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := common.Marshal(value)
	require.NoError(t, err)
	return raw
}
