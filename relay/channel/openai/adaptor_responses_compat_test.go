package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestUsesChatCompatForGLM(t *testing.T) {
	info := &relaycommon.RelayInfo{
		RelayFormat:     types.RelayFormatOpenAIResponses,
		RelayMode:       relayconstant.RelayModeResponses,
		RequestURLPath:  "/v1/responses",
		OriginModelName: "glm-5.2",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeOpenAI,
			ChannelBaseUrl:    "https://open.bigmodel.cn/api/coding/paas/v4",
			UpstreamModelName: "glm-5.2",
		},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model: "glm-5.2",
		Input: mustOpenAIAdaptorRawMessage(t, "hi"),
		Tools: mustOpenAIAdaptorRawMessage(t, []map[string]any{
			{"type": "tool_search"},
			{"type": "mcp", "name": "browser.click"},
		}),
	})
	require.NoError(t, err)

	chatReq, ok := converted.(*dto.GeneralOpenAIRequest)
	require.True(t, ok)
	assert.Equal(t, "/v1/chat/completions", info.RequestURLPath)
	assert.Equal(t, types.RelayFormatOpenAI, info.GetFinalRequestRelayFormat())
	require.Len(t, chatReq.Tools, 2)
	assert.Equal(t, "function", chatReq.Tools[0].Type)
	assert.Equal(t, "function", chatReq.Tools[1].Type)
	assert.Equal(t, "tool_search", chatReq.Tools[0].Function.Name)
	assert.Equal(t, "codex__mcp__browser_click", chatReq.Tools[1].Function.Name)
}

func mustOpenAIAdaptorRawMessage(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := common.Marshal(value)
	require.NoError(t, err)
	return raw
}
