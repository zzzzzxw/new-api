package relay

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRemoveResponsesEncryptedContentKeepsPortableHistory(t *testing.T) {
	body := mustRelayRawMessage(t, map[string]any{
		"model": "grok-4.5",
		"input": []map[string]any{
			{
				"type":              "reasoning",
				"id":                "rs_previous",
				"encrypted_content": "provider-private-value",
				"summary": []map[string]any{
					{"type": "summary_text", "text": "portable summary"},
				},
			},
			{
				"type":              "message",
				"role":              "assistant",
				"content":           "visible answer",
				"encrypted_content": "unrelated message field",
			},
			{
				"type":    "reasoning",
				"id":      "rs_summary_only",
				"summary": []map[string]any{{"type": "summary_text", "text": "already portable"}},
			},
		},
		"include": []string{"reasoning.encrypted_content"},
	})

	got, removed, err := removeResponsesEncryptedContent(body)
	require.NoError(t, err)
	require.Equal(t, 1, removed)

	assert.False(t, gjson.GetBytes(got, "input.0.encrypted_content").Exists())
	assert.Equal(t, "rs_previous", gjson.GetBytes(got, "input.0.id").String())
	assert.Equal(t, "portable summary", gjson.GetBytes(got, "input.0.summary.0.text").String())
	assert.Equal(t, "unrelated message field", gjson.GetBytes(got, "input.1.encrypted_content").String())
	assert.Equal(t, "already portable", gjson.GetBytes(got, "input.2.summary.0.text").String())
	assert.Equal(t, "reasoning.encrypted_content", gjson.GetBytes(got, "include.0").String())
}

func TestRemoveResponsesEncryptedContentLeavesRequestUnchangedWhenAbsent(t *testing.T) {
	body := []byte(`{"model":"grok-4.5","input":[{"type":"message","role":"user","content":"continue"}]}`)

	got, removed, err := removeResponsesEncryptedContent(body)
	require.NoError(t, err)
	assert.Zero(t, removed)
	assert.Equal(t, body, got)
}

func TestIsEncryptedContentDecryptionErrorRequiresMatchingBadRequest(t *testing.T) {
	matching := types.NewOpenAIError(
		errors.New("Could not decrypt the provided encrypted_content. Ensure the value is unmodified."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)
	assert.True(t, isEncryptedContentDecryptionError(matching))

	wrongStatus := types.NewOpenAIError(
		errors.New("Could not decrypt the provided encrypted_content."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusInternalServerError,
	)
	assert.False(t, isEncryptedContentDecryptionError(wrongStatus))

	otherBadRequest := types.NewOpenAIError(
		errors.New("Invalid request content."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)
	assert.False(t, isEncryptedContentDecryptionError(otherBadRequest))
}

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
