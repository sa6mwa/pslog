package pslog

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLookupEnvPrefix(t *testing.T) {
	t.Setenv("PSLOG_INT_LEVEL", "debug")

	if _, ok := lookupEnv("PSLOG_INT_", "LEVEL"); !ok {
		t.Fatalf("expected lookupEnv with prefix to find value")
	}
	if _, ok := lookupEnv("", "PSLOG_INT_LEVEL"); !ok {
		t.Fatalf("expected lookupEnv with empty prefix to find value")
	}
}

func TestParseEnvBool(t *testing.T) {
	cases := []struct {
		value string
		want  bool
		ok    bool
	}{
		{"true", true, true},
		{"1", true, true},
		{"false", false, true},
		{"0", false, true},
		{" nope ", false, false},
	}
	for _, tc := range cases {
		got, ok := parseEnvBool(tc.value)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("parseEnvBool(%q)=%v,%v want %v,%v", tc.value, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseEnvMode(t *testing.T) {
	cases := []struct {
		value string
		want  Mode
		ok    bool
	}{
		{"console", ModeConsole, true},
		{"structured", ModeStructured, true},
		{"json", ModeStructured, true},
		{" nope ", ModeConsole, false},
	}
	for _, tc := range cases {
		got, ok := parseEnvMode(tc.value)
		if ok != tc.ok || got != tc.want {
			t.Fatalf("parseEnvMode(%q)=%v,%v want %v,%v", tc.value, got, ok, tc.want, tc.ok)
		}
	}
}

func TestWriterFromEnvOutputDefaultKeepsBase(t *testing.T) {
	base := &bytes.Buffer{}
	writer, err := writerFromEnvOutput("default", base)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writer != base {
		t.Fatalf("expected base writer to be returned")
	}
}

func TestWriterFromEnvOutputStdoutStderr(t *testing.T) {
	writer, err := writerFromEnvOutput("stdout", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writer != os.Stdout {
		t.Fatalf("expected stdout writer")
	}

	writer, err = writerFromEnvOutput("stderr", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writer != os.Stderr {
		t.Fatalf("expected stderr writer")
	}
}

func TestWriterFromEnvOutputFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.log")

	writer, err := writerFromEnvOutput(path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := writer.Write([]byte("file")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	closeWriter(t, writer)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(data), "file") {
		t.Fatalf("expected file output, got %q", string(data))
	}
}

func TestWriterFromEnvOutputDefaultTee(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tee.log")

	var buf bytes.Buffer
	writer, err := writerFromEnvOutput("default+"+path, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := writer.Write([]byte("tee")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	closeWriter(t, writer)
	if !strings.Contains(buf.String(), "tee") {
		t.Fatalf("expected base writer to receive tee output")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(data), "tee") {
		t.Fatalf("expected file tee output, got %q", string(data))
	}
}

func TestWriterFromEnvOutputStdoutTee(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "stdout.log")

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() {
		os.Stdout = old
	})

	writer, err := writerFromEnvOutput("stdout+"+path, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := writer.Write([]byte("stdout")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	closeWriter(t, writer)
	_ = w.Close()
	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	_ = r.Close()

	if !strings.Contains(string(output), "stdout") {
		t.Fatalf("expected stdout output, got %q", string(output))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !strings.Contains(string(data), "stdout") {
		t.Fatalf("expected file output, got %q", string(data))
	}
}

func TestWriterFromEnvOutputErrorFallback(t *testing.T) {
	dir := t.TempDir()

	base := &bytes.Buffer{}
	writer, err := writerFromEnvOutput(dir, base)
	if err == nil {
		t.Fatalf("expected error for directory output")
	}
	if writer != base {
		t.Fatalf("expected base writer fallback")
	}
}

func closeWriter(t *testing.T, w io.Writer) {
	t.Helper()
	if c, ok := w.(interface{ Close() error }); ok {
		if err := c.Close(); err != nil {
			t.Fatalf("close writer: %v", err)
		}
	}
}
