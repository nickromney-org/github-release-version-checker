package audit

import "testing"

func TestParseWorkflowExtractsDependencies(t *testing.T) {
	repo := Repository{
		Name:     "backend-api",
		FullName: "acme/backend-api",
		Path:     "/tmp/backend-api",
		Source:   string(ModeLocal),
	}

	workflow := `name: CI
on: push
jobs:
  deploy:
    uses: acme/platform/.github/workflows/deploy.yml@main
  build:
    container:
      image: ghcr.io/acme/build:latest
    services:
      redis:
        image: redis:7.2.4
    steps:
      - name: Checkout
        uses: actions/checkout@v4
      - name: Pinned
        uses: acme/build-action@0123456789abcdef0123456789abcdef01234567
      - uses: docker://alpine:latest
      - name: Local Action
        uses: ./.github/actions/release
`

	occurrences, err := ParseWorkflow(repo, ".github/workflows/ci.yml", []byte(workflow))
	if err != nil {
		t.Fatalf("ParseWorkflow() error = %v", err)
	}

	if len(occurrences) != 7 {
		t.Fatalf("got %d occurrences, want 7", len(occurrences))
	}

	tests := map[int]struct {
		kind    Kind
		name    string
		ref     string
		refType RefType
		risk    Risk
		step    string
	}{
		5:  {KindReusableWorkflow, "acme/platform/.github/workflows/deploy.yml", "main", RefTypeLatest, RiskFloating, ""},
		8:  {KindContainer, "ghcr.io/acme/build", "latest", RefTypeLatest, RiskFloating, "container"},
		11: {KindContainer, "redis", "7.2.4", RefTypeExact, RiskReview, "redis"},
		14: {KindAction, "actions/checkout", "v4", RefTypeMajor, RiskReview, "Checkout"},
		16: {KindAction, "acme/build-action", "0123456789abcdef0123456789abcdef01234567", RefTypeSHA, RiskSafe, "Pinned"},
		17: {KindDockerAction, "alpine", "latest", RefTypeLatest, RiskFloating, "docker://alpine:latest"},
		19: {KindAction, "./.github/actions/release", "", RefTypeLocal, RiskSafe, "Local Action"},
	}

	for _, got := range occurrences {
		tt, ok := tests[got.Line]
		if !ok {
			t.Fatalf("unexpected occurrence line %d: %#v", got.Line, got)
		}
		if got.Kind != tt.kind {
			t.Errorf("occurrence line %d Kind = %q, want %q", got.Line, got.Kind, tt.kind)
		}
		if got.Name != tt.name {
			t.Errorf("occurrence line %d Name = %q, want %q", got.Line, got.Name, tt.name)
		}
		if got.Ref != tt.ref {
			t.Errorf("occurrence line %d Ref = %q, want %q", got.Line, got.Ref, tt.ref)
		}
		if got.RefType != tt.refType {
			t.Errorf("occurrence line %d RefType = %q, want %q", got.Line, got.RefType, tt.refType)
		}
		if got.Risk != tt.risk {
			t.Errorf("occurrence line %d Risk = %q, want %q", got.Line, got.Risk, tt.risk)
		}
		if got.Step != tt.step {
			t.Errorf("occurrence line %d Step = %q, want %q", got.Line, got.Step, tt.step)
		}
	}
}

func TestParseWorkflowInvalidYAML(t *testing.T) {
	repo := Repository{Name: "backend-api", Path: "/tmp/backend-api", Source: string(ModeLocal)}
	_, err := ParseWorkflow(repo, ".github/workflows/bad.yml", []byte("jobs:\n  broken: ["))
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}
