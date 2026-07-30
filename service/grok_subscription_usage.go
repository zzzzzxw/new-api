package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	grokSubscriptionTokenAuthHeader   = "xai-grok-cli"
	grokSubscriptionDefaultCLIVersion = "0.2.114"
)

func FetchGrokSubscriptionUser(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	accessToken string,
) (statusCode int, body []byte, err error) {
	return fetchGrokSubscriptionUsageEndpoint(
		ctx,
		client,
		baseURL,
		"/user",
		url.Values{"include": []string{"subscription"}},
		accessToken,
		"",
	)
}

func FetchGrokSubscriptionBilling(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	accessToken string,
	userID string,
) (statusCode int, body []byte, err error) {
	return fetchGrokSubscriptionUsageEndpoint(
		ctx,
		client,
		baseURL,
		"/billing",
		url.Values{"format": []string{"credits"}},
		accessToken,
		userID,
	)
}

func FetchGrokSubscriptionAutoTopupRule(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	accessToken string,
	userID string,
) (statusCode int, body []byte, err error) {
	return fetchGrokSubscriptionUsageEndpoint(
		ctx,
		client,
		baseURL,
		"/auto-topup-rule",
		nil,
		accessToken,
		userID,
	)
}

func fetchGrokSubscriptionUsageEndpoint(
	ctx context.Context,
	client *http.Client,
	baseURL string,
	path string,
	query url.Values,
	accessToken string,
	userID string,
) (statusCode int, body []byte, err error) {
	if client == nil {
		return 0, nil, fmt.Errorf("nil http client")
	}
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return 0, nil, fmt.Errorf("empty access token")
	}

	endpoint, err := grokSubscriptionUsageURL(baseURL, path, query)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-XAI-Token-Auth", grokSubscriptionTokenAuthHeader)
	req.Header.Set("x-grok-client-version", grokSubscriptionCLIVersion())
	req.Header.Set("x-grok-client-mode", "interactive")
	req.Header.Set("Accept", "application/json")
	if userID = strings.TrimSpace(userID); userID != "" {
		req.Header.Set("x-userid", userID)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func grokSubscriptionCLIVersion() string {
	if version := strings.TrimSpace(os.Getenv("XAI_GROK_CLI_VERSION")); version != "" {
		return version
	}
	return grokSubscriptionDefaultCLIVersion
}

func grokSubscriptionUsageURL(baseURL string, path string, query url.Values) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", fmt.Errorf("empty base URL")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid base URL")
	}

	basePath := strings.TrimRight(parsed.Path, "/")
	if !strings.HasSuffix(basePath, "/v1") {
		basePath += "/v1"
	}
	parsed.Path = basePath + "/" + strings.TrimLeft(path, "/")
	parsed.RawPath = ""
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""
	return parsed.String(), nil
}
