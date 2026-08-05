package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Non-reasoning upstreams (e.g. Kimi/DeepSeek routed via OpenAI-compatible
// channel) must normalize a client-supplied developer role back to system so
// the upstream does not reject it with "role 'developer' is not allowed".
func TestConvertOpenAIRequestNormalizesDeveloperRoleForNonReasoningModel(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-chat",
		Messages: []dto.Message{
			{Role: "developer", Content: "system prompt"},
			{Role: "user", Content: "hello"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       1, // ChannelTypeOpenAI
			UpstreamModelName: "deepseek-chat",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Equal(t, "system", convertedRequest.Messages[0].Role, "developer role should be normalized to system for non-reasoning models")
	assert.Equal(t, "user", convertedRequest.Messages[1].Role)
}

// An explicit system role must remain system for non-reasoning models.
func TestConvertOpenAIRequestKeepsSystemRoleForNonReasoningModel(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "deepseek-chat",
		Messages: []dto.Message{
			{Role: "system", Content: "system prompt"},
			{Role: "user", Content: "hello"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       1,
			UpstreamModelName: "deepseek-chat",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Equal(t, "system", convertedRequest.Messages[0].Role)
}

// Reasoning models (o-series, excluding o1-mini/preview) should still map
// system -> developer, so OpenAI receives the role it expects.
func TestConvertOpenAIRequestMapsSystemToDeveloperForReasoningModel(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "o3",
		Messages: []dto.Message{
			{Role: "system", Content: "system prompt"},
			{Role: "user", Content: "hello"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       1,
			UpstreamModelName: "o3",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Equal(t, "developer", convertedRequest.Messages[0].Role, "system role should become developer for o-series reasoning models")
}

// gpt-5 should also map system -> developer.
func TestConvertOpenAIRequestMapsSystemToDeveloperForGPT5Model(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "gpt-5",
		Messages: []dto.Message{
			{Role: "system", Content: "system prompt"},
			{Role: "user", Content: "hello"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       1,
			UpstreamModelName: "gpt-5",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Equal(t, "developer", convertedRequest.Messages[0].Role, "system role should become developer for gpt-5 models")
}

// o1-mini should keep system (it does not support developer).
func TestConvertOpenAIRequestKeepsSystemForO1Mini(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Model: "o1-mini",
		Messages: []dto.Message{
			{Role: "system", Content: "system prompt"},
			{Role: "user", Content: "hello"},
		},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       1,
			UpstreamModelName: "o1-mini",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIRequest(nil, info, request)

	require.NoError(t, err)
	convertedRequest, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Equal(t, "system", convertedRequest.Messages[0].Role, "o1-mini should keep system role")
}

// normalizeDeveloperRoleToSystem should convert ALL developer messages,
// not just the first one.
func TestNormalizeDeveloperRoleToSystemConvertsAll(t *testing.T) {
	request := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{
			{Role: "developer", Content: "first"},
			{Role: "user", Content: "msg"},
			{Role: "developer", Content: "second"},
		},
	}

	normalizeDeveloperRoleToSystem(request)

	assert.Equal(t, "system", request.Messages[0].Role)
	assert.Equal(t, "user", request.Messages[1].Role)
	assert.Equal(t, "system", request.Messages[2].Role)
}
