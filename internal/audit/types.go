package audit

import "time"

type Mode string

const (
	ModeLocal Mode = "local"
	ModeRepo  Mode = "repo"
	ModeOwner Mode = "owner"
	ModeOrg   Mode = "org"
)

type View string

const (
	ViewSummary     View = "summary"
	ViewOccurrences View = "occurrences"
)

type Format string

const (
	FormatTable    Format = "table"
	FormatJSON     Format = "json"
	FormatCSV      Format = "csv"
	FormatMarkdown Format = "markdown"
)

type FailOn string

const (
	FailOnNone     FailOn = "none"
	FailOnFloating FailOn = "floating"
)

type Kind string

const (
	KindAction           Kind = "action"
	KindReusableWorkflow Kind = "reusable-workflow"
	KindDockerAction     Kind = "docker-action"
	KindContainer        Kind = "container"
	KindAll              Kind = "all"
)

type RefType string

const (
	RefTypeNone     RefType = "none"
	RefTypeLocal    RefType = "local"
	RefTypeSHA      RefType = "sha"
	RefTypeDigest   RefType = "digest"
	RefTypeMajor    RefType = "major"
	RefTypeExact    RefType = "exact"
	RefTypeTag      RefType = "tag"
	RefTypeBranch   RefType = "branch"
	RefTypeLatest   RefType = "latest"
	RefTypeImplicit RefType = "implicit-latest"
	RefTypeUnknown  RefType = "unknown"
)

type Risk string

const (
	RiskSafe     Risk = "safe"
	RiskReview   Risk = "review"
	RiskFloating Risk = "floating"
)

type Pinning string

const (
	PinningFloating Pinning = "floating"
	PinningSemver   Pinning = "semver"
	PinningSHA      Pinning = "sha"
)

type Options struct {
	Mode              Mode      `json:"mode"`
	Path              string    `json:"path,omitempty"`
	Repo              string    `json:"repo,omitempty"`
	Owner             string    `json:"owner,omitempty"`
	Org               string    `json:"org,omitempty"`
	Host              string    `json:"host,omitempty"`
	Match             string    `json:"match,omitempty"`
	RepoFilters       []string  `json:"repo_filters,omitempty"`
	Kind              Kind      `json:"kind,omitempty"`
	Pinning           []Pinning `json:"pinning,omitempty"`
	ResolveLatest     bool      `json:"resolve_latest"`
	ResolveSHA        bool      `json:"resolve_sha"`
	CooldownDays      int       `json:"cooldown_days,omitempty"`
	IncludeContainers bool      `json:"include_containers,omitempty"`
	OnlyFloating      bool      `json:"only_floating,omitempty"`
	View              View      `json:"view,omitempty"`
	Format            Format    `json:"format,omitempty"`
	FailOn            FailOn    `json:"fail_on,omitempty"`
	MaxDepth          int       `json:"max_depth,omitempty"`
	Visibility        string    `json:"visibility,omitempty"`
	IncludeArchived   bool      `json:"include_archived,omitempty"`
	IncludeForks      bool      `json:"include_forks,omitempty"`
	Languages         []string  `json:"languages,omitempty"`
}

type Repository struct {
	Name          string `json:"name"`
	FullName      string `json:"full_name,omitempty"`
	Path          string `json:"path,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"`
	Archived      bool   `json:"archived,omitempty"`
	Fork          bool   `json:"fork,omitempty"`
	Language      string `json:"language,omitempty"`
	Visibility    string `json:"visibility,omitempty"`
	Source        string `json:"source"`
}

func (r Repository) PathOrFullName() string {
	if r.FullName != "" {
		return r.FullName
	}
	return r.Path
}

type WorkflowFile struct {
	Path    string
	Content []byte
}

type RepositoryScan struct {
	Repository Repository
	Workflows  []WorkflowFile
}

type Occurrence struct {
	Kind               Kind    `json:"kind"`
	Name               string  `json:"name"`
	UpstreamURL        string  `json:"upstream_url,omitempty"`
	Ref                string  `json:"ref,omitempty"`
	LatestRef          string  `json:"latest_ref,omitempty"`
	LatestRefURL       string  `json:"latest_ref_url,omitempty"`
	LatestAgeDays      *int    `json:"latest_age_days,omitempty"`
	LatestSHA          string  `json:"latest_sha,omitempty"`
	PinnedUses         string  `json:"pinned_uses,omitempty"`
	RefType            RefType `json:"ref_type"`
	Risk               Risk    `json:"risk"`
	Pinning            Pinning `json:"pinning,omitempty"`
	RepoName           string  `json:"repo_name"`
	RepoFullName       string  `json:"repo_full_name,omitempty"`
	RepoPath           string  `json:"repo_path,omitempty"`
	RepoDefaultBranch  string  `json:"repo_default_branch,omitempty"`
	RepoPathOrFullName string  `json:"repo_path_or_full_name"`
	WorkflowPath       string  `json:"workflow_path"`
	Job                string  `json:"job,omitempty"`
	Step               string  `json:"step,omitempty"`
	Line               int     `json:"line"`
}

type SummaryRow struct {
	Kind            Kind    `json:"kind"`
	Name            string  `json:"name"`
	UpstreamURL     string  `json:"upstream_url,omitempty"`
	Ref             string  `json:"ref,omitempty"`
	LatestRef       string  `json:"latest_ref,omitempty"`
	LatestRefURL    string  `json:"latest_ref_url,omitempty"`
	LatestAgeDays   *int    `json:"latest_age_days,omitempty"`
	LatestSHA       string  `json:"latest_sha,omitempty"`
	RefType         RefType `json:"ref_type"`
	Risk            Risk    `json:"risk"`
	Pinning         Pinning `json:"pinning,omitempty"`
	RepoCount       int     `json:"repo_count"`
	OccurrenceCount int     `json:"occurrence_count"`
}

type Metadata struct {
	GeneratedAt           time.Time `json:"generated_at"`
	Mode                  Mode      `json:"mode"`
	Path                  string    `json:"path,omitempty"`
	Repo                  string    `json:"repo,omitempty"`
	Owner                 string    `json:"owner,omitempty"`
	Org                   string    `json:"org,omitempty"`
	Host                  string    `json:"host,omitempty"`
	RepositoriesScanned   int       `json:"repositories_scanned"`
	RepositoriesWithFlows int       `json:"repositories_with_workflows"`
	WorkflowFilesScanned  int       `json:"workflow_files_scanned"`
	OccurrencesFound      int       `json:"occurrences_found"`
	WarningsCount         int       `json:"warnings_count"`
	Filters               Options   `json:"filters"`
}

type Warning struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

type Result struct {
	Metadata    Metadata     `json:"metadata"`
	Summary     []SummaryRow `json:"summary"`
	Occurrences []Occurrence `json:"occurrences"`
	Warnings    []Warning    `json:"warnings,omitempty"`
}

func (r *Result) HasFloating() bool {
	for _, occurrence := range r.Occurrences {
		if occurrence.Risk == RiskFloating {
			return true
		}
	}
	return false
}
