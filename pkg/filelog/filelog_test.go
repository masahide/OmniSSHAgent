package filelog

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"
)

func TestFileLogWriteReturnsAfterContextCancelled(t *testing.T) {
	fl := New("testapp", 1)
	fl.Close()
	if _, err := fl.Write([]byte("after close\n")); err == nil {
		t.Fatalf("Write should fail once context is cancelled")
	}
}

func TestFileLogWriteDisabledWritesToStderr(t *testing.T) {
	fl := New("stderrcase", 1)
	defer func() {
		fl.Close()
	}()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()

	origStderr := os.Stderr
	os.Stderr = writer
	defer func() {
		os.Stderr = origStderr
	}()

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 64)
		n, err := reader.Read(buf)
		if err != nil && err != io.EOF {
			t.Fatalf("failed to read from pipe: %v", err)
		}
		done <- append([]byte(nil), buf[:n]...)
	}()

	if _, err := fl.Write([]byte("async stderr\n")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	select {
	case got := <-done:
		if !bytes.Contains(got, []byte("async stderr")) {
			t.Fatalf("unexpected stderr payload: %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stderr write")
	}
}

func TestFileLogWriteEnabledCreatesFile(t *testing.T) {
	tempDir := t.TempDir()
	unsetAppData := setEnv(t, "APPDATA", tempDir)
	defer unsetAppData()
	unsetXDG := setEnv(t, "XDG_CONFIG_HOME", tempDir)
	defer unsetXDG()

	fl := New("filelogcase", 1)
	fl.SetEnable(true)

	closed := false
	defer func() {
		if !closed {
			fl.Close()
		}
	}()

	if _, err := fl.Write([]byte("file output\n")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	fl.Close()
	closed = true

	data, err := os.ReadFile(fl.FilePath)
	if err != nil {
		t.Fatalf("failed to read file %s: %v", fl.FilePath, err)
	}
	if !bytes.Contains(data, []byte("file output")) {
		t.Fatalf("file did not contain expected payload: %q", data)
	}
}

func setEnv(t *testing.T, key, value string) func() {
	t.Helper()
	prev, ok := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set env %s: %v", key, err)
	}
	return func() {
		if !ok {
			_ = os.Unsetenv(key)
			return
		}
		if err := os.Setenv(key, prev); err != nil {
			t.Fatalf("failed to restore env %s: %v", key, err)
		}
	}
}
