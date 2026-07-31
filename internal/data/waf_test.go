package data

import (
	"path/filepath"
	"testing"

	"github.com/libtnb/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/acepanel/panel/v3/internal/biz"
)

func TestNormalizePolicyStatusUsesPanelBindings(t *testing.T) {
	boundPolicyIDs := map[uint64]struct{}{
		2: {},
		3: {},
		4: {},
		5: {},
	}
	policies := []any{
		map[string]any{"id": float64(1), "version": float64(3), "applied_version": float64(3)},
		map[string]any{"id": float64(2), "version": float64(3), "applied_version": float64(2)},
		map[string]any{"id": float64(3), "version": float64(3), "applied_version": float64(3)},
		map[string]any{"id": float64(4), "version": float64(3), "applied_version": float64(2), "last_error": "load failed"},
		map[string]any{"id": float64(5), "version": float64(3)},
	}

	normalizePolicyStatus(policies, boundPolicyIDs)

	assert.Equal(t, biz.WafPolicyApplyStateSaved, policies[0].(map[string]any)["apply_status"])
	assert.Equal(t, biz.WafPolicyApplyStatePending, policies[1].(map[string]any)["apply_status"])
	assert.Equal(t, biz.WafPolicyApplyStateApplied, policies[2].(map[string]any)["apply_status"])
	assert.Equal(t, biz.WafPolicyApplyStateFailed, policies[3].(map[string]any)["apply_status"])
	assert.Equal(t, biz.WafPolicyApplyStatePending, policies[4].(map[string]any)["apply_status"])
}

func TestNormalizePolicyStatusMarksUnboundPolicySavedDespiteStaleAgentState(t *testing.T) {
	policy := map[string]any{
		"policy_id":       float64(7),
		"target_version":  float64(4),
		"applied_version": float64(4),
		"last_error":      "old error",
	}

	normalizePolicyStatus(policy, nil)

	require.Equal(t, biz.WafPolicyApplyStateSaved, policy["apply_status"])
}

func TestWithPolicyApplyStateUsesEnabledBindings(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "waf.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&biz.WafBinding{}))
	require.NoError(t, db.Create([]*biz.WafBinding{
		{WebsiteID: 1, PolicyID: 11, Enabled: true},
		{WebsiteID: 2, PolicyID: 12, Enabled: false},
	}).Error)

	policies := []any{
		map[string]any{"id": float64(11), "version": float64(2), "applied_version": float64(2)},
		map[string]any{"id": float64(12), "version": float64(2), "applied_version": float64(2)},
	}
	result, err := (&wafRepo{db: db}).withPolicyApplyState(policies)
	require.NoError(t, err)

	normalized := result.([]any)
	assert.Equal(t, biz.WafPolicyApplyStateApplied, normalized[0].(map[string]any)["apply_status"])
	assert.Equal(t, biz.WafPolicyApplyStateSaved, normalized[1].(map[string]any)["apply_status"])
}
