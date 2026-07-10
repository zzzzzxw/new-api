package common

import (
	"fmt"
	"strings"

	appcommon "github.com/QuantumNous/new-api/common"
)

const (
	requestParameterMaxStringBytes = 500
	requestParameterMaxArrayItems  = 5
	requestParameterMaxDepth       = 4
)

func (info *RelayInfo) SetRequestParametersFromValue(value any) {
	if info == nil || value == nil {
		return
	}
	info.RequestParameters = BuildRequestParametersSnapshotFromValue(value)
}

func (info *RelayInfo) SetUpstreamRequestParametersFromJSON(data []byte) {
	if info == nil || len(data) == 0 {
		return
	}
	info.UpstreamRequestParameters = BuildRequestParametersSnapshotFromJSON(data)
}

func (info *RelayInfo) SetUpstreamRequestParametersFromValue(value any) {
	if info == nil || value == nil {
		return
	}
	info.UpstreamRequestParameters = BuildRequestParametersSnapshotFromValue(value)
}

func (info *RelayInfo) SetUpstreamResponseParametersFromJSON(data []byte) {
	if info == nil || len(data) == 0 {
		return
	}
	info.UpstreamResponseParameters = BuildRequestParametersSnapshotFromJSON(data)
}

func (info *RelayInfo) SetUpstreamResponseParametersFromValue(value any) {
	if info == nil || value == nil {
		return
	}
	info.UpstreamResponseParameters = BuildRequestParametersSnapshotFromValue(value)
}

func BuildRequestParametersSnapshotFromValue(value any) map[string]interface{} {
	data, err := appcommon.Marshal(value)
	if err != nil {
		return map[string]interface{}{"_error": err.Error()}
	}
	return BuildRequestParametersSnapshotFromJSON(data)
}

func BuildRequestParametersSnapshotFromJSON(data []byte) map[string]interface{} {
	var value interface{}
	if err := appcommon.Unmarshal(data, &value); err != nil {
		return map[string]interface{}{
			"_error": err.Error(),
			"_raw":   truncateRequestParameterString(string(data)),
		}
	}
	if object, ok := value.(map[string]interface{}); ok {
		return sanitizeRequestParameterObject(object, 0)
	}
	return map[string]interface{}{"value": sanitizeRequestParameterValue("value", value, 0)}
}

func sanitizeRequestParameterObject(object map[string]interface{}, depth int) map[string]interface{} {
	out := make(map[string]interface{}, len(object))
	for key, value := range object {
		if value == nil {
			continue
		}
		out[key] = sanitizeRequestParameterValue(key, value, depth+1)
	}
	return out
}

func sanitizeRequestParameterValue(key string, value interface{}, depth int) interface{} {
	if isSensitiveRequestParameterKey(key) {
		return "[redacted]"
	}
	if depth > requestParameterMaxDepth {
		switch v := value.(type) {
		case map[string]interface{}:
			return fmt.Sprintf("[object with %d keys]", len(v))
		case []interface{}:
			return fmt.Sprintf("[array with %d items]", len(v))
		default:
			return value
		}
	}

	switch v := value.(type) {
	case string:
		return truncateRequestParameterString(v)
	case map[string]interface{}:
		return sanitizeRequestParameterObject(v, depth)
	case []interface{}:
		return sanitizeRequestParameterArray(key, v, depth)
	default:
		return v
	}
}

func sanitizeRequestParameterArray(key string, items []interface{}, depth int) interface{} {
	sanitized := make([]interface{}, 0, min(len(items), requestParameterMaxArrayItems))
	for i, item := range items {
		if i >= requestParameterMaxArrayItems {
			break
		}
		sanitized = append(sanitized, sanitizeRequestParameterValue(key, item, depth+1))
	}

	if len(items) <= requestParameterMaxArrayItems {
		return sanitized
	}

	return map[string]interface{}{
		"count": len(items),
		"items": sanitized,
	}
}

func truncateRequestParameterString(value string) string {
	if len(value) <= requestParameterMaxStringBytes {
		return value
	}
	return value[:requestParameterMaxStringBytes] + fmt.Sprintf("... [truncated, %d bytes]", len(value))
}

func isSensitiveRequestParameterKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "authorization") ||
		strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "access_token") ||
		strings.Contains(normalized, "refresh_token") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "secret") {
		return true
	}
	return normalized == "key" || normalized == "token"
}

func cloneRequestParametersSnapshot(value map[string]interface{}) map[string]interface{} {
	if value == nil {
		return nil
	}
	data, err := appcommon.Marshal(value)
	if err != nil {
		return nil
	}
	var cloned map[string]interface{}
	if err := appcommon.Unmarshal(data, &cloned); err != nil {
		return nil
	}
	return cloned
}

func (info *RelayInfo) CopyRequestParametersToUpstream() {
	if info == nil || info.RequestParameters == nil {
		return
	}
	info.UpstreamRequestParameters = cloneRequestParametersSnapshot(info.RequestParameters)
}
