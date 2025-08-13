package telemetry

import (
	"testing"
	"time"
)

// Helpers to set rate-limit timestamps and restore after test
func withLastAttemptTime(t *testing.T, ts time.Time) {
	prev := lastAttemptTime
	lastAttemptTime = ts
	t.Cleanup(func() { lastAttemptTime = prev })
}

func withLastSuccessTime(t *testing.T, ts time.Time) {
	prev := lastSuccessTime
	lastSuccessTime = ts
	t.Cleanup(func() { lastSuccessTime = prev })
}

func TestGetOrganizationMeta_UsesCustomOverrides(t *testing.T) {
	telemetryConfig := map[string]string{
		"ORGANIZATION_ID": "org-custom",
		"ACCOUNT_MAIL":    "user@example.com",
	}
	orgID, account, refresh := GetOrganizationMeta(telemetryConfig, "", "", "cluster-id", "token")

	if orgID != "org-custom" || account != "user@example.com" {
		t.Fatalf("expected custom values, got orgID=%q account=%q", orgID, account)
	}
	if refresh {
		t.Fatalf("expected refresh=false when using custom overrides")
	}
}

func TestGetOrganizationMeta_UsesCachedWhenAvailable(t *testing.T) {
	telemetryConfig := map[string]string{}
	orgID, account, refresh := GetOrganizationMeta(telemetryConfig, "org-cached", "cached@example.com", "cluster-id", "token")

	if orgID != "org-cached" || account != "cached@example.com" {
		t.Fatalf("expected cached values, got orgID=%q account=%q", orgID, account)
	}
	if refresh {
		t.Fatalf("expected refresh=false when using cached values")
	}
}

func TestGetOrganizationMeta_SkipsFetch_WhenSuccessRecent_OneMissing(t *testing.T) {
	telemetryConfig := map[string]string{}

	// Emulate recent successful fetch within 24h
	withLastSuccessTime(t, time.Now())

	orgID, account, refresh := GetOrganizationMeta(telemetryConfig, "org-cached", "", "cluster-id", "token")

	if orgID != "org-cached" || account != "" {
		t.Fatalf("expected to return cached org and empty account without fetching, got orgID=%q account=%q", orgID, account)
	}
	if refresh {
		t.Fatalf("expected refresh=false when fetch is rate-limited by last success")
	}
}

func TestGetOrganizationMeta_SkipsFetch_WhenAttemptRecent_NoCache(t *testing.T) {
	telemetryConfig := map[string]string{}

	// Emulate recent attempt within backoff window (no success)
	withLastAttemptTime(t, time.Now())

	orgID, account, refresh := GetOrganizationMeta(telemetryConfig, "", "", "cluster-id", "token")

	if orgID != "" || account != "" {
		t.Fatalf("expected empty values when no cache and fetch is rate-limited by last attempt, got orgID=%q account=%q", orgID, account)
	}
	if refresh {
		t.Fatalf("expected refresh=false when fetch is rate-limited by last attempt")
	}
}
