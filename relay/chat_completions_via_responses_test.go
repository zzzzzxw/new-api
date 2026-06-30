package relay

import (
	"testing"

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
