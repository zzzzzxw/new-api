package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelInfoScanSupportsDatabaseJSONRepresentations(t *testing.T) {
	for _, value := range []any{
		`{"is_multi_key":true,"multi_key_size":2,"multi_key_status_list":{"1":2},"multi_key_mode":"polling"}`,
		[]byte(`{"is_multi_key":true,"multi_key_size":2,"multi_key_status_list":{"1":2},"multi_key_mode":"polling"}`),
	} {
		var info ChannelInfo
		require.NoError(t, info.Scan(value))
		assert.True(t, info.IsMultiKey)
		assert.Equal(t, 2, info.MultiKeySize)
		assert.Equal(t, 2, info.MultiKeyStatusList[1])
		assert.Equal(t, constant.MultiKeyMode("polling"), info.MultiKeyMode)
	}
}

func TestChannelInfoScanTreatsEmptyDatabaseValuesAsZeroValue(t *testing.T) {
	for _, value := range []any{nil, "", []byte("")} {
		var info ChannelInfo
		require.NoError(t, info.Scan(value))
		assert.False(t, info.IsMultiKey)
		assert.Equal(t, 0, info.MultiKeySize)
	}
}

func TestChannelReasoningEffortDefaultsToEnabled(t *testing.T) {
	channel := &Channel{}
	assert.True(t, channel.GetReasoningEffortEnabled())

	disabled := false
	channel.ReasoningEffortEnabled = &disabled
	assert.False(t, channel.GetReasoningEffortEnabled())
}

func TestChannelValidatesReasoningEffortMapping(t *testing.T) {
	tests := []struct {
		name        string
		mapping     string
		errorString string
	}{
		{
			name:    "valid mapping",
			mapping: `[{"model":"glm-5.2","original_reasoning_effort":"medium","replacement_reasoning_effort":"high"}]`,
		},
		{
			name:        "mapping must be an array",
			mapping:     `{"model":"glm-5.2"}`,
			errorString: "reasoning_effort_mapping must be a valid JSON array",
		},
		{
			name:        "all fields are required",
			mapping:     `[{"model":"glm-5.2","original_reasoning_effort":"medium"}]`,
			errorString: "reasoning_effort_mapping entries require model, original_reasoning_effort, and replacement_reasoning_effort",
		},
		{
			name:        "model and original effort must be unique",
			mapping:     `[{"model":"glm-5.2","original_reasoning_effort":"medium","replacement_reasoning_effort":"high"},{"model":"glm-5.2","original_reasoning_effort":"MEDIUM","replacement_reasoning_effort":"low"}]`,
			errorString: "reasoning_effort_mapping contains duplicate model and original_reasoning_effort entries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &Channel{ReasoningEffortMapping: &tt.mapping}
			err := channel.ValidateSettings()
			if tt.errorString == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.errorString)
		})
	}
}
