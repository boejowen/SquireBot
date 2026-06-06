//go:build !windows

package onboarding

import (
	"errors"
	"strings"
	"testing"
)

// withStdin swaps the package stdin seam to a fixed string for one test and
// restores it afterward. It also resets the persistent bufio.Reader (CR-02) so
// the new source is freshly wrapped — without this reset a test would read from
// the PREVIOUS test's buffered reader.
func withStdin(t *testing.T, input string) {
	t.Helper()
	orig := stdin
	stdin = strings.NewReader(input)
	reader = nil
	t.Cleanup(func() {
		stdin = orig
		reader = nil
	})
}

// TestPromptGuildCode_ReturnsTrimmedCode: a typed code (with surrounding
// whitespace) round-trips trimmed.
func TestPromptGuildCode_ReturnsTrimmedCode(t *testing.T) {
	withStdin(t, "  MYCODE-123 \n")
	got, err := PromptGuildCode("SquireBot Setup", "Paste your guild code:")
	if err != nil {
		t.Fatalf("PromptGuildCode: %v", err)
	}
	if got != "MYCODE-123" {
		t.Fatalf("got %q, want %q", got, "MYCODE-123")
	}
}

// TestPromptGuildCode_EmptyLineCancels: an empty line (just Enter) → ErrCancelled.
func TestPromptGuildCode_EmptyLineCancels(t *testing.T) {
	withStdin(t, "\n")
	_, err := PromptGuildCode("t", "p")
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("got %v, want ErrCancelled", err)
	}
}

// TestPromptGuildCode_EOFCancels: a closed/non-tty stdin with nothing typed →
// ErrCancelled (the EOF branch).
func TestPromptGuildCode_EOFCancels(t *testing.T) {
	withStdin(t, "")
	_, err := PromptGuildCode("t", "p")
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("got %v, want ErrCancelled", err)
	}
}

// TestPickEQFolder_ExpandsEnv: "$HOME/eq" with HOME set expands to "<home>/eq".
func TestPickEQFolder_ExpandsEnv(t *testing.T) {
	t.Setenv("HOME", "/tmp/x")
	withStdin(t, "$HOME/eq\n")
	got, err := PickEQFolder("Pick your EverQuest folder")
	if err != nil {
		t.Fatalf("PickEQFolder: %v", err)
	}
	if got != "/tmp/x/eq" {
		t.Fatalf("got %q, want %q", got, "/tmp/x/eq")
	}
}

// TestPickEQFolder_ExpandsTilde: a leading "~/" expands to $HOME.
func TestPickEQFolder_ExpandsTilde(t *testing.T) {
	t.Setenv("HOME", "/home/guildie")
	withStdin(t, "~/.wine/drive_c/Project1999\n")
	got, err := PickEQFolder("Pick your EverQuest folder")
	if err != nil {
		t.Fatalf("PickEQFolder: %v", err)
	}
	if got != "/home/guildie/.wine/drive_c/Project1999" {
		t.Fatalf("got %q, want %q", got, "/home/guildie/.wine/drive_c/Project1999")
	}
}

// TestPickEQFolder_EmptyCancels: empty input → ErrCancelled.
func TestPickEQFolder_EmptyCancels(t *testing.T) {
	withStdin(t, "\n")
	_, err := PickEQFolder("t")
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("got %v, want ErrCancelled", err)
	}
}

// TestPromptsSharePersistentReader is the CR-02 regression: a SINGLE piped
// stdin carrying BOTH the guild code AND the EQ folder (as a scripted
// `printf 'CODE\n/path\n' | squirebot --setup` would deliver) must drive BOTH
// prompts in order. With the old per-call bufio.NewReader the first prompt
// buffered the folder line past its newline and the second prompt saw EOF →
// ErrCancelled. The persistent reader keeps the buffered tail available.
func TestPromptsSharePersistentReader(t *testing.T) {
	t.Setenv("HOME", "/home/guildie")
	// Two lines in ONE reader — exactly what a pipe/heredoc delivers.
	withStdin(t, "MYCODE-123\n~/.wine/drive_c/Project1999\n")

	code, err := PromptGuildCode("SquireBot Setup", "Paste your guild code:")
	if err != nil {
		t.Fatalf("PromptGuildCode: %v", err)
	}
	if code != "MYCODE-123" {
		t.Fatalf("guild code = %q, want %q", code, "MYCODE-123")
	}

	folder, err := PickEQFolder("Pick your EverQuest folder")
	if err != nil {
		t.Fatalf("PickEQFolder (second prompt lost buffered input?): %v", err)
	}
	if folder != "/home/guildie/.wine/drive_c/Project1999" {
		t.Fatalf("EQ folder = %q, want %q", folder, "/home/guildie/.wine/drive_c/Project1999")
	}
}

// TestExpandPath_BareTildeUnsetHome is the WR-01 guard: a bare "~" with no
// resolvable home dir must NOT silently leak a literal "~"; it is left verbatim
// (no second ExpandEnv pass) so the caller's ValidateFolder rejects it clearly.
func TestExpandPath_BareTildeUnsetHome(t *testing.T) {
	t.Setenv("HOME", "")
	if got := expandPath("~"); got != "~" {
		t.Fatalf("expandPath(\"~\") with unset HOME = %q, want \"~\" (left verbatim)", got)
	}
}
