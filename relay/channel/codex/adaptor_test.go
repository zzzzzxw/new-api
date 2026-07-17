package codex

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertOpenAIResponsesRequestForcesCodexUpstreamStreaming(t *testing.T) {
	streamFalse := false
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponses,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:  "gpt-test",
		Stream: &streamFalse,
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.NotNil(t, request.Stream)
	assert.True(t, *request.Stream)
}

func TestConvertOpenAIResponsesRequestPreservesCompactStreamValue(t *testing.T) {
	streamFalse := false
	info := &relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeResponsesCompact,
		ChannelMeta: &relaycommon.ChannelMeta{},
	}

	converted, err := (&Adaptor{}).ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:  "gpt-test",
		Stream: &streamFalse,
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	require.NotNil(t, request.Stream)
	assert.False(t, *request.Stream)
}

func TestSetupRequestHeaderRequestsSSEForNonStreamingCodexClient(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		IsStream:  false,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: `{"access_token":"token","account_id":"account"}`,
		},
	}
	header := make(http.Header)

	err := (&Adaptor{}).SetupRequestHeader(c, &header, info)
	require.NoError(t, err)
	assert.Equal(t, "text/event-stream", header.Get("Accept"))
}

func TestSetupRequestHeaderPreservesCompactAcceptBehavior(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", nil)

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponsesCompact,
		IsStream:  false,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: `{"access_token":"token","account_id":"account"}`,
		},
	}
	header := make(http.Header)

	err := (&Adaptor{}).SetupRequestHeader(c, &header, info)
	require.NoError(t, err)
	assert.Equal(t, "application/json", header.Get("Accept"))
}

func TestDoResponseBuffersCodexSSEForNonStreamingClient(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Set(common.RequestIdKey, "codex-buffered-test")

	body := strings.Join([]string{
		`event: response.output_text.delta`,
		`data: {"type":"response.output_text.delta","delta":"buffered text"}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","model":"gpt-test","status":"completed","usage":{"input_tokens":2,"output_tokens":3,"total_tokens":5}}}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeResponses,
		IsStream:  false,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
		},
	}

	usageValue, newAPIError := (&Adaptor{}).DoResponse(c, resp, info)
	require.Nil(t, newAPIError)
	usage, ok := usageValue.(*dto.Usage)
	require.True(t, ok)
	assert.Equal(t, 2, usage.PromptTokens)
	assert.Equal(t, 3, usage.CompletionTokens)
	assert.Equal(t, 5, usage.TotalTokens)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))
	assert.NotContains(t, recorder.Body.String(), "event:")
	assert.NotContains(t, recorder.Body.String(), "data:")
	assert.Contains(t, recorder.Body.String(), `"id":"resp_1"`)
	assert.Contains(t, recorder.Body.String(), `"text":"buffered text"`)
}
