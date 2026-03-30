package audit

import (
	"bytes"
	"strings"
	"testing"

	colour "github.com/fatih/color"
)

func TestRenderTableOccurrencesUsesAbsoluteLocalWorkflowPath(t *testing.T) {
	result := &Result{
		Metadata: Metadata{
			Mode:                ModeLocal,
			RepositoriesScanned: 1,
		},
		Occurrences: []Occurrence{
			{
				Kind:         KindAction,
				Name:         "actions/checkout",
				Ref:          "v6",
				LatestRef:    "v6.0.2",
				Pinning:      PinningSemver,
				RepoPath:     "/tmp/example-repo",
				WorkflowPath: ".github/workflows/checks.yml",
				Job:          "test",
				Step:         "Checkout",
				Line:         19,
			},
		},
	}

	var output bytes.Buffer
	if err := Render(&output, result, FormatTable, ViewOccurrences); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	got := output.String()
	if !strings.Contains(got, "/tmp/example-repo/.github/workflows/checks.yml") {
		t.Fatalf("table output did not contain absolute workflow path:\n%s", got)
	}
}

func TestRenderMarkdownOccurrencesUsesAbsoluteLocalWorkflowPath(t *testing.T) {
	result := &Result{
		Metadata: Metadata{
			Mode:                ModeLocal,
			RepositoriesScanned: 1,
		},
		Occurrences: []Occurrence{
			{
				Kind:         KindAction,
				Name:         "actions/checkout",
				Ref:          "v6",
				LatestRef:    "v6.0.2",
				Pinning:      PinningSemver,
				RepoPath:     "/tmp/example-repo",
				WorkflowPath: ".github/workflows/checks.yml",
				Job:          "test",
				Step:         "Checkout",
				Line:         19,
			},
		},
	}

	var output bytes.Buffer
	if err := Render(&output, result, FormatMarkdown, ViewOccurrences); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	got := output.String()
	if !strings.Contains(got, "[/tmp/example-repo/.github/workflows/checks.yml](/tmp/example-repo/.github/workflows/checks.yml)") {
		t.Fatalf("markdown output did not contain absolute workflow path link:\n%s", got)
	}
}

func TestRenderTableHighlightsLatestWhenCurrentDiffers(t *testing.T) {
	previous := colour.NoColor
	colour.NoColor = false
	t.Cleanup(func() { colour.NoColor = previous })

	result := &Result{
		Metadata: Metadata{
			Mode:                ModeLocal,
			RepositoriesScanned: 1,
		},
		Summary: []SummaryRow{
			{
				Kind:            KindAction,
				Name:            "actions/checkout",
				Ref:             "v6",
				LatestRef:       "v6.0.2",
				Pinning:         PinningSemver,
				OccurrenceCount: 4,
			},
			{
				Kind:            KindAction,
				Name:            "actions/setup-go",
				Ref:             "v6.3.0",
				LatestRef:       "v6.3.0",
				Pinning:         PinningSemver,
				OccurrenceCount: 1,
			},
		},
	}

	var output bytes.Buffer
	if err := Render(&output, result, FormatTable, ViewSummary); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	got := output.String()
	if !strings.Contains(got, highlightLatest("v6.0.2")) {
		t.Fatalf("expected ANSI-highlighted latest value, got:\n%s", got)
	}
	if strings.Contains(got, highlightLatest("v6.3.0")) {
		t.Fatalf("expected matching latest value to remain unhighlighted, got:\n%s", got)
	}
}
