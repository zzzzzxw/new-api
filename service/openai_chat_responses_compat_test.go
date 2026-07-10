package service

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
)

func TestApplyChannelReasoningEffortMapping(t *testing.T) {
	tests := []struct {
		name          string
		meta          *relaycommon.ChannelMeta
		model         string
		original      string
		defaultEffort string
		want          string
	}{
		{
			name:          "missing channel config keeps default",
			defaultEffort: "high",
			want:          "high",
		},
		{
			name: "disabled channel omits effort",
			meta: &relaycommon.ChannelMeta{
				ReasoningEffortEnabled: boolPointer(false),
			},
			original:      "medium",
			defaultEffort: "high",
			want:          "",
		},
		{
			name: "custom mapping overrides default normalization",
			meta: &relaycommon.ChannelMeta{
				ReasoningEffortEnabled: boolPointer(true),
				ReasoningEffortMapping: `[{"model":"glm-5.2","original_reasoning_effort":"medium","replacement_reasoning_effort":"low"}]`,
				UpstreamModelName:      "glm-5.2",
			},
			model:         "glm-5.2",
			original:      "medium",
			defaultEffort: "high",
			want:          "low",
		},
		{
			name: "client model alias does not match upstream model rule",
			meta: &relaycommon.ChannelMeta{
				ReasoningEffortEnabled: boolPointer(true),
				ReasoningEffortMapping: `[{"model":"public-glm","original_reasoning_effort":"xhigh","replacement_reasoning_effort":"max"}]`,
				UpstreamModelName:      "glm-5.2",
			},
			model:         "public-glm",
			original:      "xhigh",
			defaultEffort: "high",
			want:          "high",
		},
		{
			name: "none replacement omits effort",
			meta: &relaycommon.ChannelMeta{
				ReasoningEffortEnabled: boolPointer(true),
				ReasoningEffortMapping: `[{"model":"gpt-5.5","original_reasoning_effort":"high","replacement_reasoning_effort":"none"}]`,
				UpstreamModelName:      "gpt-5.5",
			},
			model:         "gpt-5.5",
			original:      "high",
			defaultEffort: "high",
			want:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{ChannelMeta: tt.meta}
			if tt.meta == nil {
				info.ChannelMeta = nil
			}
			assert.Equal(t, tt.want, ApplyChannelReasoningEffortMapping(info, tt.model, tt.original, tt.defaultEffort))
		})
	}
}

func boolPointer(value bool) *bool {
	return &value
}
