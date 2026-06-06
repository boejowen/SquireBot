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

// readLine reads a single line from stdin and reports whether the stream ended
// before any byte was read (EOF on a closed/non-tty stdin → cancel).
func readLine() (line string, eofEmpty bool) {
	r := bufio.NewReader(stdin)
	s, err := r.ReadString('\n')
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

// expandPath expands a leading "~" to $HOME and any $VAR references in the path.
func expandPath(p string) string {
	if p == "~" {
		if home := os.Getenv("HOME"); home != "" {
			return home
		}
	} else if strings.HasPrefix(p, "~/") {
		if home := os.Getenv("HOME"); home != "" {
			p = filepath.Join(home, p[2:])
		}
	}
	return os.ExpandEnv(p)
}
