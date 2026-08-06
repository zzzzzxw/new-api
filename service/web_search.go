package service

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

const (
	duckDuckGoInstantAnswerURL = "https://api.duckduckgo.com/"
	duckDuckGoTimeout          = 15 * time.Second
	duckDuckGoMaxResponseBytes = 1 << 20 // 1 MiB

	tavilySearchURL = "https://api.tavily.com/search"
)

// SearchWeb is the entry point used by the /v1/search controller. It
// dispatches to the configured provider and falls back to DuckDuckGo when
// the provider is not configured or its request fails.
func SearchWeb(query string, maxResults int) (*dto.WebSearchResponse, string, error) {
	setting := system_setting.GetWebSearchSetting()
	provider := setting.Provider
	if provider == system_setting.WebSearchProviderTavily && setting.TavilyApiKey != "" {
		resp, err := SearchTavily(query, maxResults, setting.TavilyApiKey)
		if err == nil {
			return resp, provider, nil
		}
		common.SysLog("tavily search failed, falling back to duckduckgo: " + err.Error())
		provider = system_setting.WebSearchProviderDuckDuckGo
	} else {
		provider = system_setting.WebSearchProviderDuckDuckGo
	}
	resp, err := SearchDuckDuckGo(query, maxResults)
	return resp, provider, err
}

// tavilyRequest/TavilyResponse mirror the Tavily search API.
// Docs: https://docs.tavily.com/documentation/api-reference/endpoint/search
type tavilyRequest struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

type tavilyResponse struct {
	Answer  string `json:"answer"`
	Results []struct {
		Title   string  `json:"title"`
		URL     string  `json:"url"`
		Content string  `json:"content"`
		Score   float64 `json:"score"`
	} `json:"results"`
}

// SearchTavily queries the Tavily search API and converts the response into
// our public dto.WebSearchResponse shape.
func SearchTavily(query string, maxResults int, apiKey string) (*dto.WebSearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	if maxResults <= 0 {
		maxResults = 5
	}

	payload, err := common.Marshal(tavilyRequest{Query: query, MaxResults: maxResults})
	if err != nil {
		return nil, fmt.Errorf("failed to build tavily request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, tavilySearchURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := GetHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tavily request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, duckDuckGoMaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read tavily response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tavily returned status %d: %s", resp.StatusCode, string(body))
	}

	var tavilyResp tavilyResponse
	if err := common.Unmarshal(body, &tavilyResp); err != nil {
		return nil, fmt.Errorf("failed to parse tavily response: %w", err)
	}

	return convertTavilyResponse(query, &tavilyResp), nil
}

func convertTavilyResponse(query string, resp *tavilyResponse) *dto.WebSearchResponse {
	result := &dto.WebSearchResponse{
		Query:  query,
		Answer: resp.Answer,
	}
	results := make([]dto.WebSearchResult, 0, len(resp.Results))
	for _, item := range resp.Results {
		text := item.Title
		if item.Content != "" {
			if text != "" {
				text += " — "
			}
			text += item.Content
		}
		if text == "" || item.URL == "" {
			continue
		}
		results = append(results, dto.WebSearchResult{Text: text, URL: item.URL})
	}
	result.Results = results
	return result
}

// duckDuckGoResponse mirrors the fields we care about from the DuckDuckGo
// Instant Answer API. The API is free and does not require a key.
type duckDuckGoResponse struct {
	Abstract       string `json:"Abstract"`
	AbstractText   string `json:"AbstractText"`
	AbstractSource string `json:"AbstractSource"`
	AbstractURL    string `json:"AbstractURL"`
	Heading        string `json:"Heading"`
	Answer         string `json:"Answer"`
	AnswerType     string `json:"AnswerType"`
	Definition     string `json:"Definition"`
	DefinitionURL  string `json:"DefinitionURL"`
	RelatedTopics  []struct {
		Text     string `json:"Text"`
		FirstURL string `json:"FirstURL"`
		Topics   []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Topics"`
	} `json:"RelatedTopics"`
	Results []struct {
		Text     string `json:"Text"`
		FirstURL string `json:"FirstURL"`
	} `json:"Results"`
}

// SearchDuckDuckGo queries the DuckDuckGo Instant Answer API and converts the
// response into our public dto.WebSearchResponse shape. maxResults limits the
// number of related/results entries returned to the caller.
func SearchDuckDuckGo(query string, maxResults int) (*dto.WebSearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	if maxResults <= 0 {
		maxResults = 5
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("no_html", "1")
	params.Set("skip_disambig", "1")

	req, err := http.NewRequest(http.MethodGet,
		duckDuckGoInstantAnswerURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", "new-api-web-search/1.0")

	client := GetHttpClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duckduckgo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("duckduckgo returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, duckDuckGoMaxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read duckduckgo response: %w", err)
	}

	var ddgResp duckDuckGoResponse
	if err := common.Unmarshal(body, &ddgResp); err != nil {
		return nil, fmt.Errorf("failed to parse duckduckgo response: %w", err)
	}

	return convertDuckDuckGoResponse(query, &ddgResp, maxResults), nil
}

func convertDuckDuckGoResponse(query string, resp *duckDuckGoResponse, maxResults int) *dto.WebSearchResponse {
	result := &dto.WebSearchResponse{
		Query:    query,
		Abstract: resp.AbstractText,
		Source:   resp.AbstractSource,
		URL:      resp.AbstractURL,
		Heading:  resp.Heading,
		Answer:   resp.Answer,
	}
	if result.Abstract == "" {
		result.Abstract = resp.Definition
	}
	if result.URL == "" {
		result.URL = resp.DefinitionURL
	}

	results := make([]dto.WebSearchResult, 0, maxResults)
	appendResult := func(text, firstURL string) {
		if len(results) >= maxResults {
			return
		}
		if text == "" || firstURL == "" {
			return
		}
		results = append(results, dto.WebSearchResult{Text: text, URL: firstURL})
	}
	for _, topic := range resp.RelatedTopics {
		if len(results) >= maxResults {
			break
		}
		if topic.Text != "" {
			appendResult(topic.Text, topic.FirstURL)
			continue
		}
		for _, sub := range topic.Topics {
			appendResult(sub.Text, sub.FirstURL)
		}
	}
	for _, item := range resp.Results {
		appendResult(item.Text, item.FirstURL)
	}
	result.Results = results
	return result
}
