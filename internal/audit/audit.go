// Package audit writes one JSON line per tool call.
package audit

import (
	"encoding/json"
	"io"
	"sync"
	"time"
)

// Entry describes one tool call.
type Entry struct {
	Time     time.Time
	Tool     string
	SQL      string // query only
	Table    string // describe_table only
	RowCount int
	Duration time.Duration
	Err      error
}

// line is the JSON shape of an Entry.
type line struct {
	Time       string `json:"time"`
	Tool       string `json:"tool"`
	SQL        string `json:"sql,omitempty"`
	Table      string `json:"table,omitempty"`
	RowCount   int    `json:"row_count"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// Logger writes entries to w. It is safe for concurrent use.
type Logger struct {
	mu sync.Mutex
	w  io.Writer
}

// New returns a Logger that writes to w.
func New(w io.Writer) *Logger {
	return &Logger{w: w}
}

// Log writes e as a single JSON line.
func (l *Logger) Log(e Entry) error {
	rec := line{
		Time:       e.Time.UTC().Format("2006-01-02T15:04:05.000Z07:00"),
		Tool:       e.Tool,
		SQL:        e.SQL,
		Table:      e.Table,
		RowCount:   e.RowCount,
		DurationMS: e.Duration.Milliseconds(),
	}
	if e.Err != nil {
		rec.Error = e.Err.Error()
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, err = l.w.Write(append(b, '\n'))
	return err
}
