package logging

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDailyWriterRotatesByDate(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 7, 20, 23, 59, 0, 0, time.Local)
	var fallback bytes.Buffer
	writer := NewDailyWriter(dir, &fallback)
	writer.now = func() time.Time { return now }
	if _, err := writer.Write([]byte("first\n")); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := writer.Write([]byte("second\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	for _, date := range []string{"20260720", "20260721"} {
		path := filepath.Join(dir, "omnisshagent-"+date+".log")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	if fallback.Len() != 0 {
		t.Fatalf("unexpected fallback: %s", fallback.String())
	}
}

func TestConfiguredLevelFiltersRecords(t *testing.T) {
	dir := t.TempDir()
	logger, closer := New(dir, slog.LevelWarn)
	logger.Info("filtered")
	logger.Warn("kept")
	if err := closer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "omnisshagent-"+time.Now().Format("20060102")+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "filtered") || !strings.Contains(string(data), "kept") {
		t.Fatalf("unexpected log contents: %s", data)
	}
}

func TestProductionLogCallsDoNotPassSensitivePayloadIdentifiers(t *testing.T) {
	root := filepath.Clean(filepath.Join("..", ".."))
	for _, dir := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, line := range strings.Split(string(data), "\n") {
				lower := strings.ToLower(line)
				if !strings.Contains(lower, "logger.") {
					continue
				}
				for _, forbidden := range []string{"privatekey", "passphrase", "payload", `"data"`, `"contents"`} {
					if strings.Contains(lower, forbidden) {
						t.Errorf("%s contains forbidden log argument %q: %s", path, forbidden, line)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
