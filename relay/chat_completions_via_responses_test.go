package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/stretchr/testify/assert"
)

func TestIsResponsesEventStreamContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{name: "plain", contentType: "text/event-stream", want: true},
		{name: "mixed case with charset", contentType: "Text/Event-Stream; charset=utf-8", want: true},
		{name: "json", contentType: "application/json", want: false},
		{name: "empty", contentType: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isResponsesEventStreamContentType(tt.contentType))
		})
	}
}

func TestShouldHandleResponsesAsStream(t *testing.T) {
	tests := []struct {
		name         string
		clientStream bool
		contentType  string
		want         bool
	}{
		{name: "client stream wins without upstream content type", clientStream: true, contentType: "application/json", want: true},
		{name: "upstream event stream", clientStream: false, contentType: "text/event-stream", want: true},
		{name: "non stream json", clientStream: false, contentType: "application/json", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldHandleResponsesAsStream(tt.clientStream, tt.contentType))
		})
	}
}

func TestShouldHandleResponsesUpstreamAsStream(t *testing.T) {
	tests := []struct {
		name         string
		apiType      int
		clientStream bool
		contentType  string
		want         bool
	}{
		{
			name:         "codex ignores missing event stream content type",
			apiType:      constant.APITypeCodex,
			clientStream: false,
			contentType:  "application/json",
			want:         true,
		},
		{
			name:         "non codex keeps json response behavior",
			apiType:      constant.APITypeOpenAI,
			clientStream: false,
			contentType:  "application/json",
			want:         false,
		},
		{
			name:         "non codex still detects event stream",
			apiType:      constant.APITypeOpenAI,
			clientStream: false,
			contentType:  "text/event-stream",
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldHandleResponsesUpstreamAsStream(tt.apiType, tt.clientStream, tt.contentType))
		})
	}
}

func TestForceCodexResponsesStream(t *testing.T) {
	tests := []struct {
		name      string
		info      *relaycommon.RelayInfo
		body      string
		wantBody  string
		wantError bool
	}{
		{
			name: "codex responses forces stream",
			info: &relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeResponses,
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiType: constant.APITypeCodex,
				},
			},
			body:     `{"model":"gpt-test","stream":false}`,
			wantBody: `{"model":"gpt-test","stream":true}`,
		},
		{
			name: "codex compact is unchanged",
			info: &relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeResponsesCompact,
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiType: constant.APITypeCodex,
				},
			},
			body:     `{"model":"gpt-test","stream":false}`,
			wantBody: `{"model":"gpt-test","stream":false}`,
		},
		{
			name: "openai responses is unchanged",
			info: &relaycommon.RelayInfo{
				RelayMode: relayconstant.RelayModeResponses,
				ChannelMeta: &relaycommon.ChannelMeta{
					ApiType: constant.APITypeOpenAI,
				},
			},
			body:     `{"model":"gpt-test","stream":false}`,
			wantBody: `{"model":"gpt-test","stream":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := forceCodexResponsesStream([]byte(tt.body), tt.info)
			if tt.wantError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.JSONEq(t, tt.wantBody, string(got))
		})
	}
}
