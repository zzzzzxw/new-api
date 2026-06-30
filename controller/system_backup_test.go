package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRestoreSystemBackupKeepsImportedChannelsQueryable(t *testing.T) {
	originalDB := model.DB
	originalSetup := constant.Setup
	originalMainDBType := common.MainDatabaseType()
	originalLogDBType := common.LogDatabaseType()
	t.Cleanup(func() {
		model.DB = originalDB
		constant.Setup = originalSetup
		common.SetDatabaseTypes(originalMainDBType, originalLogDBType)
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(systemBackupModels()...))
	model.DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	backup := &systemBackupFile{
		FormatVersion: systemBackupFormatVersion,
		Tables: map[string][]map[string]any{
			"channels": {
				{
					"id":           int64(7),
					"type":         int64(1),
					"key":          "sk-test",
					"status":       int64(common.ChannelStatusEnabled),
					"name":         "imported channel",
					"group":        "default",
					"models":       "gpt-4o",
					"channel_info": `{"is_multi_key":true,"multi_key_size":1,"multi_key_status_list":{},"multi_key_mode":"polling"}`,
				},
			},
		},
	}

	summary, err := restoreSystemBackup(backup)
	require.NoError(t, err)
	assert.Equal(t, 1, summary.ImportedRows)

	channels, err := model.GetAllChannels(0, 10, true, true)
	require.NoError(t, err)
	require.Len(t, channels, 1)
	assert.Equal(t, 7, channels[0].Id)
	assert.True(t, channels[0].ChannelInfo.IsMultiKey)
	assert.Equal(t, constant.MultiKeyModePolling, channels[0].ChannelInfo.MultiKeyMode)
}
