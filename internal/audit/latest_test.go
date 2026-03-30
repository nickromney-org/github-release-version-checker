package audit

import (
	"testing"
	"time"
)

func TestSelectLatestCandidateRespectsCooldown(t *testing.T) {
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	candidates := []latestCandidate{
		{
			Ref:         "v6.0.3",
			PublishedAt: now.Add(-2 * 24 * time.Hour),
		},
		{
			Ref:         "v6.0.2",
			PublishedAt: now.Add(-9 * 24 * time.Hour),
		},
		{
			Ref:         "v6.0.1",
			PublishedAt: now.Add(-20 * 24 * time.Hour),
		},
	}

	withCooldown := selectLatestCandidate(candidates, LatestResolveOptions{
		CooldownDays: 7,
		Now:          now,
	})
	if withCooldown.Ref != "v6.0.2" {
		t.Fatalf("Ref with cooldown = %q, want v6.0.2", withCooldown.Ref)
	}
	if withCooldown.AgeDays == nil || *withCooldown.AgeDays != 9 {
		t.Fatalf("AgeDays with cooldown = %v, want 9", withCooldown.AgeDays)
	}

	withoutCooldown := selectLatestCandidate(candidates, LatestResolveOptions{
		CooldownDays: 0,
		Now:          now,
	})
	if withoutCooldown.Ref != "v6.0.3" {
		t.Fatalf("Ref without cooldown = %q, want v6.0.3", withoutCooldown.Ref)
	}
	if withoutCooldown.AgeDays == nil || *withoutCooldown.AgeDays != 2 {
		t.Fatalf("AgeDays without cooldown = %v, want 2", withoutCooldown.AgeDays)
	}
}

func TestSelectLatestCandidatePrefersHighestSemverOverNewestPublished(t *testing.T) {
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	candidates := []latestCandidate{
		{
			Ref:         "v3.1.0-node20",
			PublishedAt: now.Add(-13 * 24 * time.Hour),
		},
		{
			Ref:         "v8.0.1",
			PublishedAt: now.Add(-19 * 24 * time.Hour),
		},
		{
			Ref:         "v4.3.0",
			PublishedAt: now.Add(-340 * 24 * time.Hour),
		},
	}

	got := selectLatestCandidate(candidates, LatestResolveOptions{
		CooldownDays: 7,
		Now:          now,
	})
	if got.Ref != "v8.0.1" {
		t.Fatalf("Ref = %q, want v8.0.1", got.Ref)
	}
	if got.AgeDays == nil || *got.AgeDays != 19 {
		t.Fatalf("AgeDays = %v, want 19", got.AgeDays)
	}
}

func TestDaysBetween(t *testing.T) {
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	then := now.Add(-49 * time.Hour)

	if got := daysBetween(now, then); got != 2 {
		t.Fatalf("daysBetween() = %d, want 2", got)
	}
}
