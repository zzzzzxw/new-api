package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCodexAuthorizationInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		code  string
		state string
	}{
		{
			name:  "callback url",
			input: "http://localhost:1455/auth/callback?code=authorization-code&state=oauth-state",
			code:  "authorization-code",
			state: "oauth-state",
		},
		{
			name:  "query string",
			input: "code=authorization-code&state=oauth-state",
			code:  "authorization-code",
			state: "oauth-state",
		},
		{
			name:  "codex cli format",
			input: "authorization-code#oauth-state",
			code:  "authorization-code",
			state: "oauth-state",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, state, err := parseCodexAuthorizationInput(test.input)
			require.NoError(t, err)
			require.Equal(t, test.code, code)
			require.Equal(t, test.state, state)
		})
	}
}
