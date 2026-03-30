package policy

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/nickromney-org/github-release-version-checker/pkg/types"
)

// PolicyResult contains the result of a policy evaluation
type PolicyResult struct {
	IsExpired      bool
	IsCritical     bool
	IsWarning      bool
	Message        string
	DaysOld        int // For days-based policies
	VersionsBehind int // For version-based policies
}

// VersionPolicy defines the interface for version expiry policies
type VersionPolicy interface {
	// Evaluate checks if a version is expired/critical
	Evaluate(
		comparison *semver.Version,
		comparisonDate time.Time,
		latest *semver.Version,
		latestDate time.Time,
		newerReleases []types.Release,
	) PolicyResult

	// Type returns the policy type
	Type() string

	// GetCriticalDays returns critical days threshold (0 if not applicable)
	GetCriticalDays() int

	// GetMaxDays returns max days threshold (0 if not applicable)
	GetMaxDays() int

	// GetMaxVersionsBehind returns max versions behind threshold (0 if not applicable)
	GetMaxVersionsBehind() int
}

// NewDaysPolicy creates a time-based policy
func NewDaysPolicy(criticalDays, maxDays int) *DaysPolicy {
	return &DaysPolicy{
		CriticalDays: criticalDays,
		MaxDays:      maxDays,
	}
}

// NewVersionsPolicy creates a version-based policy
func NewVersionsPolicy(maxMinorVersionsBehind int) *VersionsPolicy {
	return &VersionsPolicy{
		MaxMinorVersionsBehind: maxMinorVersionsBehind,
	}
}

// DaysPolicy implements time-based expiry
type DaysPolicy struct {
	CriticalDays int
	MaxDays      int
	Now          func() time.Time
}

func (p *DaysPolicy) Evaluate(
	comparison *semver.Version,
	comparisonDate time.Time,
	latest *semver.Version,
	latestDate time.Time,
	newerReleases []types.Release,
) PolicyResult {
	if len(newerReleases) == 0 {
		return PolicyResult{IsExpired: false, IsCritical: false}
	}

	// Calculate days since FIRST newer release
	firstNewer := newerReleases[0]
	now := time.Now()
	if p.Now != nil {
		now = p.Now()
	}
	daysSinceFirstNewer := int(now.Sub(firstNewer.PublishedAt).Hours() / 24)

	isExpired := daysSinceFirstNewer >= p.MaxDays
	isCritical := daysSinceFirstNewer >= p.CriticalDays && !isExpired
	isWarning := daysSinceFirstNewer > 0 && !isCritical && !isExpired

	return PolicyResult{
		IsExpired:      isExpired,
		IsCritical:     isCritical,
		IsWarning:      isWarning,
		DaysOld:        daysSinceFirstNewer,
		VersionsBehind: len(newerReleases),
		Message:        fmt.Sprintf("%d days old, %d versions behind", daysSinceFirstNewer, len(newerReleases)),
	}
}

func (p *DaysPolicy) Type() string              { return "days" }
func (p *DaysPolicy) GetCriticalDays() int      { return p.CriticalDays }
func (p *DaysPolicy) GetMaxDays() int           { return p.MaxDays }
func (p *DaysPolicy) GetMaxVersionsBehind() int { return 0 } // Not applicable

// VersionsPolicy implements version-based expiry
type VersionsPolicy struct {
	MaxMinorVersionsBehind int
}

func (p *VersionsPolicy) Evaluate(
	comparison *semver.Version,
	comparisonDate time.Time,
	latest *semver.Version,
	latestDate time.Time,
	newerReleases []types.Release,
) PolicyResult {
	if comparison.Equal(latest) {
		return PolicyResult{IsExpired: false, IsCritical: false}
	}

	// Count how many minor versions behind
	minorVersionsBehind := 0
	currentMinor := comparison.Minor()
	seenMinors := make(map[uint64]bool)

	for _, rel := range newerReleases {
		// Only count distinct minor versions within the same major version
		if rel.Version.Major() == comparison.Major() && rel.Version.Minor() > currentMinor {
			if !seenMinors[rel.Version.Minor()] {
				minorVersionsBehind++
				seenMinors[rel.Version.Minor()] = true
			}
		}

		// If major version changed, all bets are off - mark as expired
		if rel.Version.Major() > comparison.Major() {
			return PolicyResult{
				IsExpired:      true,
				IsCritical:     false,
				IsWarning:      false,
				VersionsBehind: minorVersionsBehind,
				Message:        fmt.Sprintf("major version changed (%d -> %d)", comparison.Major(), rel.Version.Major()),
			}
		}
	}

	isExpired := minorVersionsBehind > p.MaxMinorVersionsBehind
	isCritical := minorVersionsBehind == p.MaxMinorVersionsBehind
	isWarning := minorVersionsBehind > 0 && minorVersionsBehind < p.MaxMinorVersionsBehind

	return PolicyResult{
		IsExpired:      isExpired,
		IsCritical:     isCritical,
		IsWarning:      isWarning,
		VersionsBehind: minorVersionsBehind,
		Message:        fmt.Sprintf("%d minor versions behind", minorVersionsBehind),
	}
}

func (p *VersionsPolicy) Type() string              { return "versions" }
func (p *VersionsPolicy) GetCriticalDays() int      { return 0 } // Not applicable
func (p *VersionsPolicy) GetMaxDays() int           { return 0 } // Not applicable
func (p *VersionsPolicy) GetMaxVersionsBehind() int { return p.MaxMinorVersionsBehind }
