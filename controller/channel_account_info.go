package controller

import (
	"net"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
)

const zhipuCodingAPIPath = "/api/coding/paas/v4"

func isZhipuAccountInfoChannel(channel *model.Channel) bool {
	if channel == nil {
		return false
	}
	if channel.Type == constant.ChannelTypeZhipu || channel.Type == constant.ChannelTypeZhipu_v4 {
		return true
	}

	rawBaseURL := strings.TrimSpace(channel.GetBaseURL())
	if rawBaseURL == "" {
		return false
	}
	if !strings.Contains(rawBaseURL, "://") {
		rawBaseURL = "https://" + rawBaseURL
	}
	parsed, err := url.Parse(rawBaseURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		host, _, _ = net.SplitHostPort(strings.ToLower(parsed.Host))
	}
	if host != "open.bigmodel.cn" {
		return false
	}

	path := "/" + strings.Trim(strings.ToLower(parsed.EscapedPath()), "/")
	return path == zhipuCodingAPIPath || strings.HasPrefix(path, zhipuCodingAPIPath+"/")
}
