package advancedcustom

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdvancedCustomTaskAdaptorUsesConfiguredRoute(t *testing.T) {
	info := advancedCustomVideoRelayInfo("https://example.com/root", dto.AdvancedCustomRoute{
		IncomingPath: "/v1/video/generations",
		UpstreamPath: "/api/v3/models/{model}/tasks?region=cn",
		Converter:    dto.AdvancedCustomConverterNone,
		Auth: &dto.AdvancedCustomRouteAuth{
			Type:  dto.AdvancedCustomAuthTypeQuery,
			Name:  "access_token",
			Value: "{api_key}",
		},
	})

	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	requestURL, err := adaptor.BuildRequestURL(info)
	require.NoError(t, err)

	parsedURL, err := url.Parse(requestURL)
	require.NoError(t, err)
	assert.Equal(t, "https://example.com/root/api/v3/models/seedance-2-5-260628/tasks", parsedURL.Scheme+"://"+parsedURL.Host+parsedURL.Path)
	assert.Equal(t, "cn", parsedURL.Query().Get("region"))
	assert.Equal(t, "test-key", parsedURL.Query().Get("access_token"))
}

func TestAdvancedCustomTaskAdaptorBuildsSeedancePayload(t *testing.T) {
	info := advancedCustomVideoRelayInfo("https://example.com", dto.AdvancedCustomRoute{
		IncomingPath: "/v1/video/generations",
		UpstreamPath: "/api/v3/contents/generations/tasks",
	})
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Model:   "seedance-2-5-260628",
		Prompt:  "a paper plane flying over Shanghai",
		Seconds: "8",
		Metadata: map[string]any{
			"resolution":     "1080p",
			"ratio":          "16:9",
			"generate_audio": true,
			"watermark":      false,
			"content": []any{
				map[string]any{
					"type": "image_url",
					"role": "first_frame",
					"image_url": map[string]any{
						"url": "https://example.com/first.png",
					},
				},
			},
		},
	})

	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	body, err := adaptor.BuildRequestBody(ctx, info)
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(data, &payload))
	assert.Equal(t, "seedance-2-5-260628", payload["model"])
	assert.Equal(t, "1080p", payload["resolution"])
	assert.Equal(t, float64(8), payload["duration"])
	content, ok := payload["content"].([]any)
	require.True(t, ok)
	require.Len(t, content, 2)
	assert.Equal(t, "image_url", content[0].(map[string]any)["type"])
	assert.Equal(t, "first_frame", content[0].(map[string]any)["role"])
	assert.Equal(t, "text", content[1].(map[string]any)["type"])
}

func TestAdvancedCustomTaskAdaptorPollsConfiguredRoute(t *testing.T) {
	service.InitHttpClient()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		assert.Equal(t, http.MethodGet, req.Method)
		assert.Equal(t, "/models/seedance-2-5-260628/tasks/upstream-task-42", req.URL.Path)
		assert.Equal(t, "cn", req.URL.Query().Get("region"))
		assert.Equal(t, "Token test-key", req.Header.Get("X-API-Key"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"upstream-task-42","status":"running"}`))
	}))
	defer server.Close()

	info := advancedCustomVideoRelayInfo(server.URL, dto.AdvancedCustomRoute{
		IncomingPath: "/v1/video/generations",
		UpstreamPath: "/models/{model}/tasks?region=cn",
		Auth: &dto.AdvancedCustomRouteAuth{
			Type:  dto.AdvancedCustomAuthTypeHeader,
			Name:  "X-API-Key",
			Value: "Token {api_key}",
		},
	})
	info.RequestURLPath = ""
	info.UpstreamModelName = ""

	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	resp, err := adaptor.FetchTask(server.URL, "test-key", map[string]any{
		"task_id": "upstream-task-42",
		"model":   "seedance-2-5-260628",
	}, "")
	require.NoError(t, err)
	require.NotNil(t, resp)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAdvancedCustomTaskAdaptorRejectsUnsupportedConverter(t *testing.T) {
	info := advancedCustomVideoRelayInfo("https://example.com", dto.AdvancedCustomRoute{
		IncomingPath: "/v1/video/generations",
		UpstreamPath: "/tasks",
		Converter:    dto.AdvancedCustomConverterOpenAIChatCompletionsToAnthropicMessages,
	})

	adaptor := &TaskAdaptor{}
	adaptor.Init(info)
	_, err := adaptor.BuildRequestURL(info)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "converter")
}

func advancedCustomVideoRelayInfo(baseURL string, route dto.AdvancedCustomRoute) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RequestURLPath: "/v1/video/generations",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:       constant.ChannelTypeAdvancedCustom,
			ChannelBaseUrl:    baseURL,
			ApiKey:            "test-key",
			UpstreamModelName: "seedance-2-5-260628",
			ChannelOtherSettings: dto.ChannelOtherSettings{
				AdvancedCustom: &dto.AdvancedCustomConfig{
					Routes: []dto.AdvancedCustomRoute{route},
				},
			},
		},
	}
}
