package logging

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestSetupCreatesLogDir confirms that Setup creates the platform log directory
// if it does not already exist, and that the returned directory matches the
// per-platform default (Phase 25: %LOCALAPPDATA%\SquireBot on Windows,
// $XDG_STATE_HOME/squirebot elsewhere).
func TestSetupCreatesLogDir(t *testing.T) {
	tmp := t.TempDir()
	var want string
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", tmp)
		want = filepath.Join(tmp, "SquireBot")
	} else {
		t.Setenv("XDG_STATE_HOME", tmp)
		want = filepath.Join(tmp, "squirebot")
	}

	logger, dir := Setup()
	if logger == nil {
		t.Fatal("Setup returned nil logger")
	}

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

// TestDefaultLogDir_XDGState (Phase 25 / LNX-02 / D-05) verifies the platform
// branch in defaultLogDir(): Windows stays on %LOCALAPPDATA%\SquireBot; on Unix
// logs resolve under $XDG_STATE_HOME/squirebot, falling back to
// ~/.local/state/squirebot when $XDG_STATE_HOME is unset.
func TestDefaultLogDir_XDGState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Setenv("LOCALAPPDATA", `C:\Users\Test\AppData\Local`)
		got := defaultLogDir()
		want := filepath.Join(`C:\Users\Test\AppData\Local`, "SquireBot")
		if got != want {
			t.Fatalf("defaultLogDir() = %q; want %q", got, want)
		}
		return
	}

	// Unix branch 1: $XDG_STATE_HOME set → .../squirebot.
	state := t.TempDir()
	t.Setenv("XDG_STATE_HOME", state)
	if got, want := defaultLogDir(), filepath.Join(state, "squirebot"); got != want {
		t.Fatalf("defaultLogDir() with XDG_STATE_HOME = %q; want %q", got, want)
	}

	// Unix branch 2: $XDG_STATE_HOME unset → ~/.local/state/squirebot.
	t.Setenv("XDG_STATE_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	if got, want := defaultLogDir(), filepath.Join(home, ".local", "state", "squirebot"); got != want {
		t.Fatalf("defaultLogDir() fallback = %q; want %q", got, want)
	}
	if strings.Contains(defaultLogDir(), "SquireBot") {
		t.Errorf("defaultLogDir() = %q contains \"SquireBot\"; the Unix path must use lowercase \"squirebot\"", defaultLogDir())
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
