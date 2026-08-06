package dto

// WebSearchRequest is the request body for POST /v1/search.
type WebSearchRequest struct {
	// Query is the search keyword, required.
	Query string `json:"query"`
	// MaxResults limits the number of related results returned (default 5, max 20).
	MaxResults *int `json:"max_results,omitempty"`
}

// WebSearchResult is a single related search hit.
type WebSearchResult struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// WebSearchResponse is the unified response returned by POST /v1/search.
type WebSearchResponse struct {
	Query    string            `json:"query"`
	Heading  string            `json:"heading,omitempty"`
	Abstract string            `json:"abstract,omitempty"`
	Source   string            `json:"source,omitempty"`
	URL      string            `json:"url,omitempty"`
	Answer   string            `json:"answer,omitempty"`
	Results  []WebSearchResult `json:"results"`
}
