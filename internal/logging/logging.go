package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DailyWriter struct {
	dir      string
	now      func() time.Time
	fallback io.Writer
	mu       sync.Mutex
	date     string
	file     *os.File
}

func NewDailyWriter(dir string, fallback io.Writer) *DailyWriter {
	return &DailyWriter{dir: dir, now: time.Now, fallback: fallback}
}

func (w *DailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	date := w.now().Format("20060102")
	if w.file == nil || date != w.date {
		if w.file != nil {
			_ = w.file.Close()
		}
		if err := os.MkdirAll(w.dir, 0700); err != nil {
			return w.fallback.Write(p)
		}
		path := filepath.Join(w.dir, "omnisshagent-"+date+".log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return w.fallback.Write(p)
		}
		w.file, w.date = f, date
	}
	return w.file.Write(p)
}

func (w *DailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func New(dir string, level slog.Level) (*slog.Logger, io.Closer) {
	writer := NewDailyWriter(dir, os.Stderr)
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level})
	return slog.New(handler), writer
}

func SafeError(operation string, err error) string {
	return fmt.Sprintf("%s: %v", operation, err)
}
