//go:build !windows

package onboarding

// Phase 25 / D-02 (LNX-04): the Linux watcher is headless — onboarding is a CLI
// flow over stdin, NOT a Win32 dialog (that path stays Windows-only) and NOT a
// browser/HTTP onboarding surface (the carried HARD CONSTRAINT — this package
// opens no network listener of any kind). These are the real stdin prompts that
// replace the former unsupported placeholders.
//
// CONTRACT NOTE (see dialog.go): this package is a THIN UI layer — it prints the
// prompt + returns the typed value. It does NOT validate the guild code against
// the backend and does NOT run eqfind.ValidateFolder; the CALLER
// (app.runOnboarding / pickAndSaveEQFolder) does both and re-prompts on failure.
// An empty line or EOF returns ErrCancelled so the caller's errors.Is branches
// still fire.

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// stdin is the input source for the prompts. It defaults to os.Stdin; tests
// inject a strings.Reader to drive the prompts deterministically.
var stdin io.Reader = os.Stdin

// reader is ONE persistent bufio.Reader over stdin for the lifetime of the
// onboarding session (CR-02). bufio reads the underlying stream in CHUNKS, so a
// fresh bufio.Reader per prompt would let the FIRST ReadString buffer bytes past
// its newline (the EQ-folder line on piped/scripted stdin) and then DISCARD that
// tail when the next prompt allocated a new reader. Holding a single reader keeps
// those buffered bytes available to the following prompt. Interactive TTY input
// (one line delivered at a time) happened to survive the old per-call wrapping,
// but `printf 'CODE\n/path\n' | squirebot --setup` did not. Tests that swap
// `stdin` MUST reset this via resetReaderForTest so the new source is wrapped.
var reader *bufio.Reader

// stdinReader returns the lazily-initialised persistent reader over the current
// stdin seam.
func stdinReader() *bufio.Reader {
	if reader == nil {
		reader = bufio.NewReader(stdin)
	}
	return reader
}

// readLine reads a single line from the persistent stdin reader and reports
// whether the stream ended before any byte was read (EOF on a closed/non-tty
// stdin → cancel).
func readLine() (line string, eofEmpty bool) {
	s, err := stdinReader().ReadString('\n')
	if err != nil && s == "" {
		return "", true // EOF/closed stream with nothing typed
	}
	return s, false
}

// PromptGuildCode prints the title + prompt to stderr and reads one line from
// stdin, trimmed. An empty line or EOF returns ErrCancelled. The code itself is
// NEVER echoed back or logged (V7) — it is only returned to the caller.
func PromptGuildCode(title, prompt string) (string, error) {
	fmt.Fprintln(os.Stderr, title)
	fmt.Fprint(os.Stderr, prompt+" ")
	line, eof := readLine()
	if eof {
		return "", ErrCancelled
	}
	code := strings.TrimSpace(line)
	if code == "" {
		return "", ErrCancelled
	}
	return code, nil
}

// PickEQFolder prints the prompt to stderr and reads one line (a filesystem
// path) from stdin, trimmed. A leading "~" expands to $HOME and embedded
// $VAR / ${VAR} (e.g. $WINEPREFIX) are expanded via os.ExpandEnv. An empty line
// or EOF returns ErrCancelled. Validation is the caller's job.
func PickEQFolder(title string) (string, error) {
	fmt.Fprint(os.Stderr, title+": ")
	line, eof := readLine()
	if eof {
		return "", ErrCancelled
	}
	p := strings.TrimSpace(line)
	if p == "" {
		return "", ErrCancelled
	}
	return expandPath(p), nil
}

// expandPath resolves $VAR / ${VAR} references and a leading "~" in a typed
// path. WR-01: env expansion runs FIRST (one pass), THEN a leading tilde is
// resolved EXACTLY ONCE — never re-expanding the joined result. $HOME is
// resolved via os.UserHomeDir() (which honors $HOME on Unix); if it is
// unavailable, a bare "~" or "~/..." is left verbatim rather than silently
// leaking a literal "~" through a second ExpandEnv pass — the caller's
// ValidateFolder then rejects it with a clear "no such directory" error. No
// command execution (no backticks/$( ) handling — os.ExpandEnv does not run a
// shell).
func expandPath(p string) string {
	p = os.ExpandEnv(p)
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p // can't resolve ~; leave the path untouched (no literal-~ leak surprise)
	}
	switch {
	case p == "~":
		return home
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(home, p[2:])
	}
	return p
}
