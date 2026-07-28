package groksubscription

import (
	"os"
	"strings"
)

var ModelList = []string{
	"grok-4.5",
	"grok-4.3",
	"grok-build-0.1",
	"grok-composer-2.5-fast",
	"grok-4.20-0309-reasoning",
	"grok-4.20-0309-non-reasoning",
	"grok-4.20-multi-agent-0309",
}

const (
	ChannelName           = "grok-subscription"
	DefaultGrokCLIVersion = "0.2.93"
	GrokUserAgent         = "new-api-grok/1.0"
)

func GrokCLIVersion() string {
	if version := strings.TrimSpace(os.Getenv("XAI_GROK_CLI_VERSION")); version != "" {
		return version
	}
	return DefaultGrokCLIVersion
}
