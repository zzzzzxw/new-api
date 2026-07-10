package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestParametersSnapshotKeepsReasoningAndRedactsSecrets(t *testing.T) {
	snapshot := BuildRequestParametersSnapshotFromJSON([]byte(`{
		"model":"gpt-5",
		"reasoning_effort":"high",
		"reasoning":{"effort":"high"},
		"api_key":"sk-test",
		"messages":[
			{"role":"user","content":"hello"}
		]
	}`))

	require.NotNil(t, snapshot)
	assert.Equal(t, "gpt-5", snapshot["model"])
	assert.Equal(t, "high", snapshot["reasoning_effort"])

	reasoning, ok := snapshot["reasoning"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "high", reasoning["effort"])
	assert.Equal(t, "[redacted]", snapshot["api_key"])
	assert.Contains(t, snapshot, "messages")
}

func TestBuildRequestParametersSnapshotSummarizesLargeArrays(t *testing.T) {
	snapshot := BuildRequestParametersSnapshotFromJSON([]byte(`{
		"input":[1,2,3,4,5,6],
		"reasoning":{"effort":"medium"}
	}`))

	input, ok := snapshot["input"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 6, input["count"])

	items, ok := input["items"].([]interface{})
	require.True(t, ok)
	assert.Len(t, items, 5)
}
