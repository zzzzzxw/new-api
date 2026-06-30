package controller

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	systemBackupFormatVersion = 1
	systemBackupMaxSize       = 256 << 20
)

type systemBackupFile struct {
	FormatVersion int                         `json:"format_version"`
	App           string                      `json:"app"`
	Version       string                      `json:"version"`
	DatabaseType  string                      `json:"database_type"`
	ExportedAt    int64                       `json:"exported_at"`
	Tables        map[string][]map[string]any `json:"tables"`
	TableCounts   map[string]int              `json:"table_counts"`
}

type systemBackupImportSummary struct {
	ImportedTables int            `json:"imported_tables"`
	ImportedRows   int            `json:"imported_rows"`
	TableCounts    map[string]int `json:"table_counts"`
}

func systemBackupModels() []any {
	return []any{
		&model.Channel{},
		&model.Token{},
		&model.User{},
		&model.PasskeyCredential{},
		&model.Option{},
		&model.Redemption{},
		&model.Ability{},
		&model.Log{},
		&model.Midjourney{},
		&model.TopUp{},
		&model.QuotaData{},
		&model.Task{},
		&model.Model{},
		&model.Vendor{},
		&model.PrefillGroup{},
		&model.Setup{},
		&model.TwoFA{},
		&model.TwoFABackupCode{},
		&model.Checkin{},
		&model.SubscriptionPlan{},
		&model.SubscriptionOrder{},
		&model.UserSubscription{},
		&model.SubscriptionPreConsumeRecord{},
		&model.CustomOAuthProvider{},
		&model.UserOAuthBinding{},
		&model.PerfMetric{},
		&model.SystemInstance{},
		&model.SystemTask{},
		&model.SystemTaskLock{},
		&model.CasbinRule{},
		&model.AuthzRole{},
	}
}

func systemBackupTableNames(db *gorm.DB) ([]string, map[string]struct{}, error) {
	tables := make([]string, 0, len(systemBackupModels()))
	allowed := make(map[string]struct{}, len(systemBackupModels()))
	for _, item := range systemBackupModels() {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(item); err != nil {
			return nil, nil, err
		}
		tableName := stmt.Schema.Table
		tables = append(tables, tableName)
		allowed[tableName] = struct{}{}
	}
	return tables, allowed, nil
}

func systemBackupQuoteIdentifier(db *gorm.DB, name string) (string, error) {
	if strings.TrimSpace(name) == "" || strings.Contains(name, "\x00") {
		return "", errors.New("invalid table name")
	}
	if strings.Contains(name, ".") {
		return "", errors.New("qualified table names are not supported")
	}
	quote := "`"
	if db.Dialector.Name() == string(common.DatabaseTypePostgreSQL) {
		quote = `"`
	}
	return quote + strings.ReplaceAll(name, quote, quote+quote) + quote, nil
}

func systemBackupRows(db *gorm.DB, tableName string) ([]map[string]any, error) {
	rows, err := db.Table(tableName).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		scanTargets := make([]any, len(columns))
		for i := range values {
			scanTargets[i] = &values[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, err
		}

		record := make(map[string]any, len(columns))
		for i, column := range columns {
			value := values[i]
			switch typed := value.(type) {
			case []byte:
				record[column] = string(typed)
			case sql.RawBytes:
				record[column] = string(typed)
			case time.Time:
				record[column] = typed.Format(time.RFC3339Nano)
			default:
				record[column] = typed
			}
		}
		result = append(result, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func ExportSystemBackup(c *gin.Context) {
	tableNames, _, err := systemBackupTableNames(model.DB)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	backup := systemBackupFile{
		FormatVersion: systemBackupFormatVersion,
		App:           "new-api",
		Version:       common.Version,
		DatabaseType:  string(common.MainDatabaseType()),
		ExportedAt:    time.Now().Unix(),
		Tables:        make(map[string][]map[string]any, len(tableNames)),
		TableCounts:   make(map[string]int, len(tableNames)),
	}

	for _, tableName := range tableNames {
		if !model.DB.Migrator().HasTable(tableName) {
			continue
		}
		rows, err := systemBackupRows(model.DB, tableName)
		if err != nil {
			common.ApiError(c, fmt.Errorf("export table %s failed: %w", tableName, err))
			return
		}
		backup.Tables[tableName] = rows
		backup.TableCounts[tableName] = len(rows)
	}

	payload, err := common.Marshal(backup)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	filename := fmt.Sprintf("new-api-backup-%s.json", time.Now().Format("20060102-150405"))
	c.Header("Content-Type", "application/json")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "application/json", payload)
}

func ImportSystemBackup(c *gin.Context) {
	if constant.Setup && c.GetInt("role") < common.RoleRootUser {
		common.ApiErrorMsg(c, "only root users can import backups")
		return
	}

	if strings.ToUpper(strings.TrimSpace(c.PostForm("confirm"))) != "IMPORT" {
		common.ApiErrorMsg(c, "confirmation text is required")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if file.Size <= 0 || file.Size > systemBackupMaxSize {
		common.ApiErrorMsg(c, "backup file is empty or too large")
		return
	}

	backup, err := readSystemBackupFile(file)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	summary, err := restoreSystemBackup(backup)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, summary)
}

func readSystemBackupFile(file *multipart.FileHeader) (*systemBackupFile, error) {
	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer src.Close()

	var backup systemBackupFile
	if err := common.DecodeJson(io.LimitReader(src, systemBackupMaxSize+1), &backup); err != nil {
		return nil, fmt.Errorf("invalid backup file: %w", err)
	}
	if backup.FormatVersion != systemBackupFormatVersion {
		return nil, fmt.Errorf("unsupported backup format version: %d", backup.FormatVersion)
	}
	if backup.Tables == nil {
		return nil, errors.New("backup file does not contain tables")
	}
	for tableName, rows := range backup.Tables {
		for i, row := range rows {
			backup.Tables[tableName][i] = normalizeSystemBackupRow(row)
		}
	}
	return &backup, nil
}

func normalizeSystemBackupRow(row map[string]any) map[string]any {
	normalized := make(map[string]any, len(row))
	for key, value := range row {
		normalized[key] = normalizeSystemBackupValue(value)
	}
	return normalized
}

func normalizeSystemBackupValue(value any) any {
	switch typed := value.(type) {
	case float64:
		const maxExactJSONInt = float64(1<<53 - 1)
		if math.Trunc(typed) == typed && typed <= maxExactJSONInt && typed >= -maxExactJSONInt {
			return int64(typed)
		}
		return typed
	default:
		return typed
	}
}

func restoreSystemBackup(backup *systemBackupFile) (systemBackupImportSummary, error) {
	tableNames, allowedTables, err := systemBackupTableNames(model.DB)
	if err != nil {
		return systemBackupImportSummary{}, err
	}

	for tableName := range backup.Tables {
		if _, ok := allowedTables[tableName]; !ok {
			return systemBackupImportSummary{}, fmt.Errorf("backup contains unsupported table: %s", tableName)
		}
	}

	summary := systemBackupImportSummary{
		TableCounts: make(map[string]int, len(backup.Tables)),
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		for i := len(tableNames) - 1; i >= 0; i-- {
			tableName := tableNames[i]
			if !tx.Migrator().HasTable(tableName) {
				continue
			}
			quoted, err := systemBackupQuoteIdentifier(tx, tableName)
			if err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM " + quoted).Error; err != nil {
				return fmt.Errorf("clear table %s failed: %w", tableName, err)
			}
		}

		for _, tableName := range tableNames {
			rows := backup.Tables[tableName]
			if len(rows) == 0 {
				continue
			}
			if err := tx.Table(tableName).CreateInBatches(rows, 200).Error; err != nil {
				return fmt.Errorf("restore table %s failed: %w", tableName, err)
			}
			summary.ImportedTables++
			summary.ImportedRows += len(rows)
			summary.TableCounts[tableName] = len(rows)
		}
		if err := resetSystemBackupSequences(tx, tableNames); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return systemBackupImportSummary{}, err
	}

	refreshSystemAfterBackupImport()
	return summary, nil
}

func resetSystemBackupSequences(tx *gorm.DB, tableNames []string) error {
	if tx.Dialector.Name() != string(common.DatabaseTypePostgreSQL) {
		return nil
	}
	for _, tableName := range tableNames {
		if !tx.Migrator().HasColumn(tableName, "id") {
			continue
		}
		quoted, err := systemBackupQuoteIdentifier(tx, tableName)
		if err != nil {
			return err
		}
		sqlText := fmt.Sprintf(`
DO $$
DECLARE
	seq_name text;
	max_id bigint;
BEGIN
	SELECT pg_get_serial_sequence('%s', 'id') INTO seq_name;
	IF seq_name IS NOT NULL THEN
		EXECUTE 'SELECT COALESCE(MAX(id), 0) FROM %s' INTO max_id;
		PERFORM setval(seq_name, GREATEST(max_id, 1), max_id > 0);
	END IF;
END $$;`, strings.ReplaceAll(tableName, "'", "''"), quoted)
		if err := tx.Exec(sqlText).Error; err != nil {
			return fmt.Errorf("reset sequence for %s failed: %w", tableName, err)
		}
	}
	return nil
}

func refreshSystemAfterBackupImport() {
	model.InitOptionMap()
	model.CheckSetup()
	model.InitChannelCache()
}
