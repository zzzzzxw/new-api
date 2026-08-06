package system_setting

import "github.com/QuantumNous/new-api/setting/config"

// Web search provider identifiers. The empty string and "duckduckgo" both
// mean the built-in free DuckDuckGo Instant Answer fallback.
const (
	WebSearchProviderDuckDuckGo = "duckduckgo"
	WebSearchProviderTavily     = "tavily"
)

// WebSearchSetting configures the /v1/search endpoint.
// DB keys: web_search_setting.provider, web_search_setting.tavily_api_key
type WebSearchSetting struct {
	// Provider selects the search backend. Empty means DuckDuckGo (free,
	// no key required) which is also the fallback when the configured
	// provider fails or is misconfigured.
	Provider string `json:"provider"`
	// TavilyApiKey is required when Provider is "tavily".
	TavilyApiKey string `json:"tavily_api_key"`
}

var defaultWebSearchSetting = WebSearchSetting{
	Provider:     WebSearchProviderDuckDuckGo,
	TavilyApiKey: "",
}

func init() {
	config.GlobalConfig.Register("web_search_setting", &defaultWebSearchSetting)
}

func GetWebSearchSetting() *WebSearchSetting {
	return &defaultWebSearchSetting
}
