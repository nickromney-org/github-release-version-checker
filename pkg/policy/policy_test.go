package policy

import (
	"testing"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/nickromney-org/github-release-version-checker/pkg/types"
)

func makeReleaseAt(ver string, daysAgo int, now time.Time) types.Release {
	return types.Release{
		Version:     semver.MustParse(ver),
		PublishedAt: now.AddDate(0, 0, -daysAgo),
		URL:         "https://github.com/test/test/releases/tag/" + ver,
	}
}

func TestDaysPolicy_Evaluate(t *testing.T) {
	now := time.Date(2026, time.March, 30, 12, 0, 0, 0, time.UTC)
	policy := &DaysPolicy{
		CriticalDays: 12,
		MaxDays:      30,
		Now: func() time.Time {
			return now
		},
	}

	tests := []struct {
		name          string
		comparison    string
		newerReleases []types.Release
		wantExpired   bool
		wantCritical  bool
		wantWarning   bool
		wantDaysOld   int
	}{
		{
			name:          "no newer releases",
			comparison:    "2.329.0",
			newerReleases: []types.Release{},
			wantExpired:   false,
			wantCritical:  false,
			wantWarning:   false,
		},
		{
			name:          "5 days old - warning",
			comparison:    "2.328.0",
			newerReleases: []types.Release{makeReleaseAt("2.329.0", 5, now)},
			wantExpired:   false,
			wantCritical:  false,
			wantWarning:   true,
			wantDaysOld:   5,
		},
		{
			name:          "12 days old - critical",
			comparison:    "2.328.0",
			newerReleases: []types.Release{makeReleaseAt("2.329.0", 12, now)},
			wantExpired:   false,
			wantCritical:  true,
			wantWarning:   false,
			wantDaysOld:   12,
		},
		{
			name:          "20 days old - still critical",
			comparison:    "2.328.0",
			newerReleases: []types.Release{makeReleaseAt("2.329.0", 20, now)},
			wantExpired:   false,
			wantCritical:  true,
			wantWarning:   false,
			wantDaysOld:   20,
		},
		{
			name:          "30 days old - expired",
			comparison:    "2.327.0",
			newerReleases: []types.Release{makeReleaseAt("2.328.0", 30, now)},
			wantExpired:   true,
			wantCritical:  false,
			wantWarning:   false,
			wantDaysOld:   30,
		},
		{
			name:          "35 days old - expired",
			comparison:    "2.327.0",
			newerReleases: []types.Release{makeReleaseAt("2.328.0", 35, now)},
			wantExpired:   true,
			wantCritical:  false,
			wantWarning:   false,
			wantDaysOld:   35,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.Evaluate(
				semver.MustParse(tt.comparison),
				now,
				semver.MustParse("2.329.0"),
				now,
				tt.newerReleases,
			)

			if result.IsExpired != tt.wantExpired {
				t.Errorf("IsExpired = %v, want %v", result.IsExpired, tt.wantExpired)
			}
			if result.IsCritical != tt.wantCritical {
				t.Errorf("IsCritical = %v, want %v", result.IsCritical, tt.wantCritical)
			}
			if result.IsWarning != tt.wantWarning {
				t.Errorf("IsWarning = %v, want %v", result.IsWarning, tt.wantWarning)
			}
			if tt.wantDaysOld > 0 && result.DaysOld != tt.wantDaysOld {
				t.Errorf("DaysOld = %v, want %v", result.DaysOld, tt.wantDaysOld)
			}
		})
	}
}

func TestVersionsPolicy_Evaluate(t *testing.T) {
	now := time.Date(2026, time.March, 30, 12, 0, 0, 0, time.UTC)
	policy := &VersionsPolicy{
		MaxMinorVersionsBehind: 3,
	}

	tests := []struct {
		name               string
		comparison         string
		latest             string
		newerReleases      []types.Release
		wantExpired        bool
		wantCritical       bool
		wantWarning        bool
		wantVersionsBehind int
	}{
		{
			name:               "on latest version",
			comparison:         "1.34.0",
			latest:             "1.34.0",
			newerReleases:      []types.Release{},
			wantExpired:        false,
			wantCritical:       false,
			wantWarning:        false,
			wantVersionsBehind: 0,
		},
		{
			name:       "1 minor version behind - warning",
			comparison: "1.33.0",
			latest:     "1.34.0",
			newerReleases: []types.Release{
				makeReleaseAt("1.34.0", 5, now),
			},
			wantExpired:        false,
			wantCritical:       false,
			wantWarning:        true,
			wantVersionsBehind: 1,
		},
		{
			name:       "2 minor versions behind - warning",
			comparison: "1.32.0",
			latest:     "1.34.0",
			newerReleases: []types.Release{
				makeReleaseAt("1.34.0", 5, now),
				makeReleaseAt("1.33.0", 35, now),
			},
			wantExpired:        false,
			wantCritical:       false,
			wantWarning:        true,
			wantVersionsBehind: 2,
		},
		{
			name:       "3 minor versions behind - critical",
			comparison: "1.31.0",
			latest:     "1.34.0",
			newerReleases: []types.Release{
				makeReleaseAt("1.34.0", 5, now),
				makeReleaseAt("1.33.0", 35, now),
				makeReleaseAt("1.32.0", 65, now),
			},
			wantExpired:        false,
			wantCritical:       true,
			wantWarning:        false,
			wantVersionsBehind: 3,
		},
		{
			name:       "4 minor versions behind - expired",
			comparison: "1.30.0",
			latest:     "1.34.0",
			newerReleases: []types.Release{
				makeReleaseAt("1.34.0", 5, now),
				makeReleaseAt("1.33.0", 35, now),
				makeReleaseAt("1.32.0", 65, now),
				makeReleaseAt("1.31.0", 95, now),
			},
			wantExpired:        true,
			wantCritical:       false,
			wantWarning:        false,
			wantVersionsBehind: 4,
		},
		{
			name:       "major version changed - expired",
			comparison: "1.34.0",
			latest:     "2.0.0",
			newerReleases: []types.Release{
				makeReleaseAt("2.0.0", 5, now),
			},
			wantExpired:  true,
			wantCritical: false,
			wantWarning:  false,
		},
		{
			name:       "multiple patches same minor - counts as 1",
			comparison: "1.32.0",
			latest:     "1.34.0",
			newerReleases: []types.Release{
				makeReleaseAt("1.34.2", 1, now),
				makeReleaseAt("1.34.1", 5, now),
				makeReleaseAt("1.34.0", 10, now),
				makeReleaseAt("1.33.5", 20, now),
				makeReleaseAt("1.33.0", 35, now),
			},
			wantExpired:        false,
			wantCritical:       false,
			wantWarning:        true,
			wantVersionsBehind: 2, // Only 1.33 and 1.34
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.Evaluate(
				semver.MustParse(tt.comparison),
				now,
				semver.MustParse(tt.latest),
				now,
				tt.newerReleases,
			)

			if result.IsExpired != tt.wantExpired {
				t.Errorf("IsExpired = %v, want %v", result.IsExpired, tt.wantExpired)
			}
			if result.IsCritical != tt.wantCritical {
				t.Errorf("IsCritical = %v, want %v", result.IsCritical, tt.wantCritical)
			}
			if result.IsWarning != tt.wantWarning {
				t.Errorf("IsWarning = %v, want %v", result.IsWarning, tt.wantWarning)
			}
			if tt.wantVersionsBehind > 0 && result.VersionsBehind != tt.wantVersionsBehind {
				t.Errorf("VersionsBehind = %v, want %v", result.VersionsBehind, tt.wantVersionsBehind)
			}
		})
	}
}

func TestNewDaysPolicy(t *testing.T) {
	policy := NewDaysPolicy(12, 30)
	if policy.Type() != "days" {
		t.Errorf("Type() = %v, want days", policy.Type())
	}
	if policy.CriticalDays != 12 {
		t.Errorf("CriticalDays = %v, want 12", policy.CriticalDays)
	}
	if policy.MaxDays != 30 {
		t.Errorf("MaxDays = %v, want 30", policy.MaxDays)
	}
}

func TestNewVersionsPolicy(t *testing.T) {
	policy := NewVersionsPolicy(3)
	if policy.Type() != "versions" {
		t.Errorf("Type() = %v, want versions", policy.Type())
	}
	if policy.MaxMinorVersionsBehind != 3 {
		t.Errorf("MaxMinorVersionsBehind = %v, want 3", policy.MaxMinorVersionsBehind)
	}
}

func TestDaysPolicy_Getters(t *testing.T) {
	policy := &DaysPolicy{
		CriticalDays: 12,
		MaxDays:      30,
	}

	if policy.GetCriticalDays() != 12 {
		t.Errorf("GetCriticalDays() = %v, want 12", policy.GetCriticalDays())
	}
	if policy.GetMaxDays() != 30 {
		t.Errorf("GetMaxDays() = %v, want 30", policy.GetMaxDays())
	}
	if policy.Type() != "days" {
		t.Errorf("Type() = %v, want days", policy.Type())
	}
}

func TestVersionsPolicy_Getters(t *testing.T) {
	policy := &VersionsPolicy{
		MaxMinorVersionsBehind: 3,
	}

	if policy.GetCriticalDays() != 0 {
		t.Errorf("GetCriticalDays() = %v, want 0", policy.GetCriticalDays())
	}
	if policy.GetMaxDays() != 0 {
		t.Errorf("GetMaxDays() = %v, want 0", policy.GetMaxDays())
	}
	if policy.Type() != "versions" {
		t.Errorf("Type() = %v, want versions", policy.Type())
	}
}
