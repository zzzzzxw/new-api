package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertDuckDuckGoResponse_BasicAbstract(t *testing.T) {
	body := `{
		"Abstract": "ignored",
		"AbstractText": "Go is a programming language.",
		"AbstractSource": "Wikipedia",
		"AbstractURL": "https://en.wikipedia.org/wiki/Go_(programming_language)",
		"Heading": "Go",
		"Answer": "",
		"RelatedTopics": [
			{"Text": "Go official site", "FirstURL": "https://go.dev"},
			{"Topics": [
				{"Text": "Go tutorial", "FirstURL": "https://go.dev/tour"},
				{"Text": "Go playground", "FirstURL": "https://go.dev/play"}
			]},
			{"Text": "Go modules", "FirstURL": "https://go.dev/ref/mod"}
		],
		"Results": [
			{"Text": "GitHub - golang", "FirstURL": "https://github.com/golang"}
		]
	}`
	var ddgResp duckDuckGoResponse
	require.NoError(t, common.Unmarshal([]byte(body), &ddgResp))

	got := convertDuckDuckGoResponse("golang", &ddgResp, 5)

	assert.Equal(t, "golang", got.Query)
	assert.Equal(t, "Go is a programming language.", got.Abstract)
	assert.Equal(t, "Wikipedia", got.Source)
	assert.Equal(t, "https://en.wikipedia.org/wiki/Go_(programming_language)", got.URL)
	assert.Equal(t, "Go", got.Heading)
	require.Len(t, got.Results, 5)
	assert.Equal(t, "Go official site", got.Results[0].Text)
	assert.Equal(t, "https://go.dev/tour", got.Results[1].URL)
	assert.Equal(t, "Go modules", got.Results[3].Text)
	assert.Equal(t, "GitHub - golang", got.Results[4].Text)
}

func TestConvertDuckDuckGoResponse_MaxResults(t *testing.T) {
	var ddgResp duckDuckGoResponse
	ddgResp.RelatedTopics = []struct {
		Text     string `json:"Text"`
		FirstURL string `json:"FirstURL"`
		Topics   []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Topics"`
	}{
		{Text: "a", FirstURL: "https://a.example"},
		{Text: "b", FirstURL: "https://b.example"},
		{Text: "c", FirstURL: "https://c.example"},
	}

	got := convertDuckDuckGoResponse("q", &ddgResp, 2)
	assert.Len(t, got.Results, 2)

	got = convertDuckDuckGoResponse("q", &ddgResp, 10)
	assert.Len(t, got.Results, 3)
}

func TestConvertDuckDuckGoResponse_DefinitionFallback(t *testing.T) {
	body := `{
		"AbstractText": "",
		"Definition": "golang: the Go programming language.",
		"DefinitionURL": "https://en.wiktionary.org/wiki/golang",
		"RelatedTopics": [],
		"Results": []
	}`
	var ddgResp duckDuckGoResponse
	require.NoError(t, common.Unmarshal([]byte(body), &ddgResp))

	got := convertDuckDuckGoResponse("golang meaning", &ddgResp, 5)
	assert.Equal(t, "golang: the Go programming language.", got.Abstract)
	assert.Equal(t, "https://en.wiktionary.org/wiki/golang", got.URL)
	assert.Empty(t, got.Results)
}

func TestSearchDuckDuckGo_EmptyQuery(t *testing.T) {
	_, err := SearchDuckDuckGo("   ", 5)
	require.Error(t, err)
}

func TestConvertTavilyResponse(t *testing.T) {
	body := `{
		"answer": "OpenAI recently announced new models.",
		"results": [
			{
				"title": "OpenAI News",
				"url": "https://openai.com/news",
				"content": "Latest updates from OpenAI.",
				"score": 0.98
			},
			{
				"title": "TechCrunch coverage",
				"url": "https://techcrunch.com/openai",
				"content": "",
				"score": 0.87
			},
			{
				"title": "",
				"url": "",
				"content": "skip me",
				"score": 0.5
			}
		]
	}`
	var tavilyResp tavilyResponse
	require.NoError(t, common.Unmarshal([]byte(body), &tavilyResp))

	got := convertTavilyResponse("openai news", &tavilyResp)

	assert.Equal(t, "openai news", got.Query)
	assert.Equal(t, "OpenAI recently announced new models.", got.Answer)
	require.Len(t, got.Results, 2)
	assert.Equal(t, "OpenAI News — Latest updates from OpenAI.", got.Results[0].Text)
	assert.Equal(t, "https://openai.com/news", got.Results[0].URL)
	assert.Equal(t, "TechCrunch coverage", got.Results[1].Text)
}

func TestSearchTavily_EmptyQuery(t *testing.T) {
	_, err := SearchTavily("  ", 5, "tvly-test")
	require.Error(t, err)
}
