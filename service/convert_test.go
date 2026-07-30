package service

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeToOpenAIRequest_MixedAssistantMessage_textAndToolUse(t *testing.T) {
	// Given: an assistant ClaudeMessage with both text and tool_use content blocks
	textContent := "Here is the result of the calculation:"
	toolInput := map[string]any{
		"expression": "2+2",
	}

	claudeMsg := dto.ClaudeMessage{
		Role: "assistant",
		Content: []any{
			map[string]any{
				"type": "text",
				"text": textContent,
			},
			map[string]any{
				"type":  "tool_use",
				"id":    "toolu_01A09q90qw90",
				"name":  "calculator",
				"input": toolInput,
			},
		},
	}

	claudeRequest := dto.ClaudeRequest{
		Model:    "claude-sonnet-4-20250514",
		Messages: []dto.ClaudeMessage{claudeMsg},
	}

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-sonnet-4-20250514",
		},
		OriginModelName: "claude-sonnet-4-20250514",
	}

	// When: converting the Claude request to an OpenAI request
	openAIRequest, err := ClaudeToOpenAIRequest(claudeRequest, info)

	// Then: the conversion succeeds and the message contains both text content and tool calls
	require.NoError(t, err)
	require.NotNil(t, openAIRequest)
	require.Len(t, openAIRequest.Messages, 1)

	openAIMsg := openAIRequest.Messages[0]
	assert.Equal(t, "assistant", openAIMsg.Role)

	// Verify text content is preserved
	contents := openAIMsg.ParseContent()
	require.NotEmpty(t, contents, "message should have content (text)")

	var hasText bool
	for _, c := range contents {
		if c.Type == dto.ContentTypeText && c.Text == textContent {
			hasText = true
			break
		}
	}
	assert.True(t, hasText, "text content should be preserved when tool_use is also present")

	toolCalls := openAIMsg.ParseToolCalls()
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "toolu_01A09q90qw90", toolCalls[0].ID)
	assert.Equal(t, "calculator", toolCalls[0].Function.Name)
	assert.JSONEq(t, `{"expression":"2+2"}`, toolCalls[0].Function.Arguments)
}
