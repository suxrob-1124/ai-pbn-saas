package legacy

import "time"

// Mode определяет режим запуска импортера.
type Mode string

const (
	ModeDryRun Mode = "dry-run"
	ModeApply  Mode = "apply"
)

// ManifestRow описывает одну строку CSV-манифеста.
type ManifestRow struct {
	RowNumber       int
	ProjectName     string
	OwnerEmail      string
	ProjectCountry  string
	ProjectLanguage string
	DomainURL       string
	MainKeyword     string
	ExcludeDomains  string
	ServerID        string
}

// BatchConfig задает параметры батч-обработки.
type BatchConfig struct {
	BatchSize   int
	BatchNumber int
}

// RunOptions определяет параметры запуска импортера.
type RunOptions struct {
	ManifestPath string
	ServerDir    string
	Mode         Mode
	Batch        BatchConfig
	Force        bool
	DecodeSource string
	DecodeOnly   bool
}

// Summary агрегирует статистику по репорту импорта.
type Summary struct {
	Processed int `json:"processed"`
	Success   int `json:"success"`
	Failed    int `json:"failed"`
	Warned    int `json:"warned"`
	Decoded   int `json:"decoded,omitempty"`
	Updated   int `json:"updated,omitempty"`
	Skipped   int `json:"skipped,omitempty"`
	Unchanged int `json:"unchanged,omitempty"`
}

// RowReport содержит результат обработки строки манифеста.
type RowReport struct {
	RowNumber      int      `json:"row_number"`
	ProjectName    string   `json:"project_name"`
	OwnerEmail     string   `json:"owner_email"`
	DomainURL      string   `json:"domain_url"`
	Status         string   `json:"status"`
	Actions        []string `json:"actions,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
	Error          string   `json:"error,omitempty"`
	FileCount      int      `json:"file_count,omitempty"`
	TotalSizeBytes int64    `json:"total_size_bytes,omitempty"`
}

// Report описывает итоговый JSON-отчет импорта.
type Report struct {
	Mode         Mode        `json:"mode"`
	ManifestPath string      `json:"manifest_path"`
	ServerDir    string      `json:"server_dir"`
	BatchSize    int         `json:"batch_size"`
	BatchNumber  int         `json:"batch_number"`
	StartedAt    time.Time   `json:"started_at"`
	FinishedAt   time.Time   `json:"finished_at"`
	Summary      Summary     `json:"summary"`
	Rows         []RowReport `json:"rows"`
}
