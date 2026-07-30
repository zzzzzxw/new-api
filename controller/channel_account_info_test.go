package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
)

func TestIsZhipuAccountInfoChannel(t *testing.T) {
	tests := []struct {
		name     string
		channel  *model.Channel
		expected bool
	}{
		{
			name:     "native zhipu channel",
			channel:  &model.Channel{Type: constant.ChannelTypeZhipu},
			expected: true,
		},
		{
			name:     "native zhipu v4 channel",
			channel:  &model.Channel{Type: constant.ChannelTypeZhipu_v4},
			expected: true,
		},
		{
			name: "openai compatible zhipu coding endpoint",
			channel: &model.Channel{
				Type:    constant.ChannelTypeOpenAI,
				BaseURL: stringPointer("https://open.bigmodel.cn/api/coding/paas/v4"),
			},
			expected: true,
		},
		{
			name: "zhipu coding endpoint with trailing path",
			channel: &model.Channel{
				Type:    constant.ChannelTypeCustom,
				BaseURL: stringPointer("open.bigmodel.cn/api/coding/paas/v4/"),
			},
			expected: true,
		},
		{
			name: "spoofed zhipu host",
			channel: &model.Channel{
				Type:    constant.ChannelTypeOpenAI,
				BaseURL: stringPointer("https://open.bigmodel.cn.evil.example/api/coding/paas/v4"),
			},
			expected: false,
		},
		{
			name: "different bigmodel endpoint",
			channel: &model.Channel{
				Type:    constant.ChannelTypeOpenAI,
				BaseURL: stringPointer("https://open.bigmodel.cn/api/paas/v4"),
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isZhipuAccountInfoChannel(test.channel))
		})
	}
}

func stringPointer(value string) *string {
	return &value
}
