package inventory

import "time"

type Mode string

const (
	ModeDryRun Mode = "dry-run"
	ModeApply  Mode = "apply"
)

type ProbeStatus string

const (
	ProbeFound            ProbeStatus = "found"
	ProbeNotFound         ProbeStatus = "not_found"
	ProbeAmbiguous        ProbeStatus = "ambiguous"
	ProbePermissionDenied ProbeStatus = "permission_denied"
	ProbeError            ProbeStatus = "error"
)

type Target struct {
	Alias   string
	Host    string
	Port    int
	User    string
	KeyPath string
}

type RunOptions struct {
	ManifestPath string
	TargetServer string
	Mode         Mode
	BatchSize    int
	BatchNumber  int
	Concurrency  int
	Retries      int
	RetryDelay   time.Duration
	JitterMin    time.Duration
	JitterMax    time.Duration
	Target       Target
}

type ProbeResult struct {
	Status        ProbeStatus
	PublishedPath string
	SiteOwner     string
	Message       string
	Candidates    []string
}

type RowReport struct {
	RowNumber       int         `json:"row_number"`
	DomainURL       string      `json:"domain_url"`
	ServerID        string      `json:"server_id"`
	NormalizedHost  string      `json:"normalized_host"`
	ProbeStatus     ProbeStatus `json:"probe_status"`
	InventoryStatus string      `json:"inventory_status,omitempty"`
	PublishedPath   string      `json:"published_path,omitempty"`
	SiteOwner       string      `json:"site_owner,omitempty"`
	Attempts        int         `json:"attempts"`
	ApplyAction     string      `json:"apply_action,omitempty"`
	Warnings        []string    `json:"warnings,omitempty"`
	Error           string      `json:"error,omitempty"`
}

type Summary struct {
	Processed         int `json:"processed"`
	Success           int `json:"success"`
	Warned            int `json:"warned"`
	Failed            int `json:"failed"`
	Found             int `json:"found"`
	NotFound          int `json:"not_found"`
	Ambiguous         int `json:"ambiguous"`
	PermissionDenied  int `json:"permission_denied"`
	DomainMissingInDB int `json:"domain_missing_in_db"`
	AppliedUpdated    int `json:"applied_updated"`
	AppliedSkipped    int `json:"applied_skipped"`
}

type Report struct {
	Mode         Mode        `json:"mode"`
	ManifestPath string      `json:"manifest_path"`
	TargetServer string      `json:"target_server"`
	StartedAt    time.Time   `json:"started_at"`
	FinishedAt   time.Time   `json:"finished_at"`
	Summary      Summary     `json:"summary"`
	Rows         []RowReport `json:"rows"`
}
