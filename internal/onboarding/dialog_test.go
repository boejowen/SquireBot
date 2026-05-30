package onboarding

import (
	"errors"
	"runtime"
	"testing"
)

// TestErrVars asserts the package's sentinel errors are non-nil and distinct.
// This is the build-green + error-contract check the CI runs on any platform; it
// does NOT drive the UI (a real DialogBox needs a desktop session).
func TestErrVars(t *testing.T) {
	if ErrCancelled == nil {
		t.Fatal("ErrCancelled is nil")
	}
	if ErrUnsupported == nil {
		t.Fatal("ErrUnsupported is nil")
	}
	if errors.Is(ErrCancelled, ErrUnsupported) || errors.Is(ErrUnsupported, ErrCancelled) {
		t.Fatal("ErrCancelled and ErrUnsupported must be distinct sentinels")
	}
}

// TestNonWindows_Unsupported asserts the !windows stubs return ErrUnsupported. On
// Windows, the interactive dialog paths are skipped (they require a desktop
// session; they are validated by the Plan 03/04 onboarding smoke + the human
// ship-gate, not by headless CI).
func TestNonWindows_Unsupported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("interactive dialog requires a desktop session; validated by the Plan 03/04 smoke + ship-gate")
	}
	if _, err := PromptGuildCode("SquireBot", "Paste your guild code:"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("PromptGuildCode off-Windows = %v, want ErrUnsupported", err)
	}
	if _, err := PickEQFolder("Select your EverQuest folder"); !errors.Is(err, ErrUnsupported) {
		t.Errorf("PickEQFolder off-Windows = %v, want ErrUnsupported", err)
	}
}
