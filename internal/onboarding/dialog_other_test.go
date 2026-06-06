//go:build !windows

package onboarding

import (
	"errors"
	"strings"
	"testing"
)

// withStdin swaps the package stdin seam to a fixed string for one test and
// restores it afterward.
func withStdin(t *testing.T, input string) {
	t.Helper()
	orig := stdin
	stdin = strings.NewReader(input)
	t.Cleanup(func() { stdin = orig })
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
