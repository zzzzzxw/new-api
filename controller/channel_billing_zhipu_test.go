package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractZhipuDashboardBalanceFromRemainingField(t *testing.T) {
	body := []byte(`{
		"code": 200,
		"data": {
			"quotaLimits": [
				{"name": "weekly", "remainingQuota": 123.5},
				{"name": "mcp", "available": "45"}
			]
		}
	}`)

	metric, err := extractZhipuDashboardBalance(body)

	require.NoError(t, err)
	require.True(t, metric.Found)
	assert.Equal(t, 123.5, metric.Value)
}

func TestExtractZhipuDashboardBalanceFromLimitAndUsage(t *testing.T) {
	body := []byte(`{
		"data": {
			"limits": [
				{"limit": 1000, "used": 250},
				{"totalQuota": "200", "usage": "50"}
			]
		}
	}`)

	metric, err := extractZhipuDashboardBalance(body)

	require.NoError(t, err)
	require.True(t, metric.Found)
	assert.Equal(t, 750.0, metric.Value)
}

func TestExtractZhipuDashboardBalanceIgnoresUsageOnly(t *testing.T) {
	body := []byte(`{
		"data": {
			"modelUsage": [
				{"model": "glm", "usage": 100, "cost": 20}
			]
		}
	}`)

	metric, err := extractZhipuDashboardBalance(body)

	require.NoError(t, err)
	assert.False(t, metric.Found)
}
