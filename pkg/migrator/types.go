package migrator

import (
	"time"
)

// Config configures the migration run.
type Config struct {
	SourceDir      string
	DestRoot       string
	DryRun         bool
	AutoConfirm    bool
	UpdateExisting bool
	NoTUI          bool
}

// EventType defines migration phase events.
type EventType string

const (
	EventScanStart     EventType = "scan_start"
	EventScanProgress  EventType = "scan_progress"
	EventScanDone      EventType = "scan_done"
	EventPlanItem      EventType = "plan_item"
	EventConfirmReady  EventType = "confirm_ready"
	EventMoveStart     EventType = "move_start"
	EventMoveDone      EventType = "move_done"
	EventMoveDuplicate EventType = "move_duplicate"
	EventMoveSkip      EventType = "move_skip"
	EventMoveError     EventType = "move_error"
	EventUpdateHTML    EventType = "update_html"
	EventCleanArchive  EventType = "clean_archive"
	EventDone          EventType = "done"
)

// ProgressEvent conveys real-time status during migration.
type ProgressEvent struct {
	Type        EventType
	Current     int
	Total       int
	MovieID     string
	Actress     string
	Title       string
	SourcePath  string
	TargetPath  string
	Message     string
	Detail      string
	Err         error
	SummaryData *Summary
}

// Summary stores the outcome of the migration run.
type Summary struct {
	TotalDiscovered int           `json:"total_discovered"`
	OrganizedCount  int           `json:"organized_count"`
	DuplicateCount  int           `json:"duplicate_count"`
	SkippedCount    int           `json:"skipped_count"`
	ErrorCount      int           `json:"error_count"`
	UpdatedHTMLNum  int           `json:"updated_html_num"`
	Duration        time.Duration `json:"duration"`
	SkippedDetails  []string      `json:"skipped_details,omitempty"`
	Errors          []string      `json:"errors,omitempty"`
}
