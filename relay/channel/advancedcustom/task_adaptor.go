package advancedcustom

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	taskdoubao "github.com/QuantumNous/new-api/relay/channel/task/doubao"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// TaskAdaptor adds asynchronous video support to Advanced Custom channels.
// The configured route controls the upstream URL and authentication, while the
// Doubao task adaptor handles the Seedance request and response protocol.
type TaskAdaptor struct {
	taskdoubao.TaskAdaptor
	info *relaycommon.RelayInfo
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.TaskAdaptor.Init(info)
	a.info = info
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	route, err := a.videoRoute(info)
	if err != nil {
		return "", err
	}
	return (&Adaptor{route: route}).routeURL(info)
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	route, err := a.videoRoute(info)
	if err != nil {
		return err
	}
	setTaskRequestHeaders(req, route, info.ApiKey)
	return nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	if a.info == nil || a.info.ChannelMeta == nil {
		return nil, fmt.Errorf("advanced custom channel info is missing")
	}

	info := *a.info
	channelMeta := *a.info.ChannelMeta
	channelMeta.ChannelBaseUrl = baseURL
	channelMeta.ApiKey = key
	if modelName, ok := body["model"].(string); ok && modelName != "" {
		channelMeta.UpstreamModelName = modelName
	}
	info.ChannelMeta = &channelMeta

	submitURL, err := a.BuildRequestURL(&info)
	if err != nil {
		return nil, err
	}
	parsedURL, err := url.Parse(submitURL)
	if err != nil {
		return nil, err
	}
	basePath := strings.TrimRight(parsedURL.Path, "/")
	baseEscapedPath := strings.TrimRight(parsedURL.EscapedPath(), "/")
	parsedURL.Path = basePath + "/" + taskID
	parsedURL.RawPath = baseEscapedPath + "/" + url.PathEscape(taskID)

	req, err := http.NewRequest(http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, err
	}
	route, err := a.videoRoute(&info)
	if err != nil {
		return nil, err
	}
	setTaskRequestHeaders(req, route, key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) videoRoute(info *relaycommon.RelayInfo) (dto.AdvancedCustomRoute, error) {
	if info == nil || info.ChannelMeta == nil {
		return dto.AdvancedCustomRoute{}, fmt.Errorf("advanced custom channel info is missing")
	}
	config := info.ChannelOtherSettings.AdvancedCustom
	if config == nil {
		return dto.AdvancedCustomRoute{}, fmt.Errorf("advanced_custom is required")
	}
	if err := config.Validate(); err != nil {
		return dto.AdvancedCustomRoute{}, err
	}

	requestPath := strings.Split(info.RequestURLPath, "?")[0]
	if requestPath == "" {
		requestPath = "/v1/video/generations"
	}
	route, ok := config.MatchPath(requestPath)
	if !ok {
		return dto.AdvancedCustomRoute{}, fmt.Errorf("advanced custom channel does not support request path: %s", requestPath)
	}
	converter := strings.TrimSpace(route.Converter)
	if converter != "" && converter != dto.AdvancedCustomConverterNone {
		return dto.AdvancedCustomRoute{}, fmt.Errorf("converter %q does not support video generation requests", converter)
	}
	return route, nil
}

func setTaskRequestHeaders(req *http.Request, route dto.AdvancedCustomRoute, apiKey string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if route.Auth == nil {
		req.Header.Set("Authorization", "Bearer "+apiKey)
		return
	}
	if strings.TrimSpace(route.Auth.Type) == dto.AdvancedCustomAuthTypeHeader {
		req.Header.Set(strings.TrimSpace(route.Auth.Name), applyAuthTemplate(route.Auth.Value, apiKey))
	}
}

var _ channel.TaskAdaptor = (*TaskAdaptor)(nil)
var _ channel.OpenAIVideoConverter = (*TaskAdaptor)(nil)
