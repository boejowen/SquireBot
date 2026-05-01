package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSetupCreatesLogDir confirms that Setup creates the %LOCALAPPDATA%\SquireBot
// directory if it does not already exist, and that the returned directory matches.
func TestSetupCreatesLogDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("LOCALAPPDATA", tmp)

	logger, dir := Setup()
	if logger == nil {
		t.Fatal("Setup returned nil logger")
	}

	want := filepath.Join(tmp, "SquireBot")
	if dir != want {
		t.Fatalf("logDir = %q; want %q", dir, want)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("log dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("log dir path %q is not a directory", dir)
	}
}

// TestSetupAtWritesJSON uses the internal helper so the test can close the
// lumberjack handle before t.TempDir() cleanup — Windows otherwise blocks the
// rmdir with "file in use".
func TestSetupAtWritesJSON(t *testing.T) {
	logDir := filepath.Join(t.TempDir(), "SquireBot")

	logger, closer, err := setupAt(logDir)
	if err != nil {
		t.Fatalf("setupAt: %v", err)
	}
	logger.Info("smoke test", "k", "v")
	if err := closer.Close(); err != nil {
		t.Fatalf("close rotator: %v", err)
	}

	logPath := filepath.Join(logDir, "squirebot.log")
	contents, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if len(contents) == 0 {
		t.Fatal("log file is empty")
	}
	s := string(contents)
	if !strings.Contains(s, `"msg":"smoke test"`) {
		t.Fatalf("log missing expected msg field; got: %s", s)
	}
	if !strings.Contains(s, `"k":"v"`) {
		t.Fatalf("log missing expected attr; got: %s", s)
	}
	// AddSource: true should produce a "source" key in each record.
	if !strings.Contains(s, `"source"`) {
		t.Errorf("log missing source field (AddSource:true expected); got: %s", s)
	}
}
