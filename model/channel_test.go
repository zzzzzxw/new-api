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
