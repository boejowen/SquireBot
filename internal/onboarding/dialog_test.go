package onboarding

import (
	"errors"
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

// NOTE (Phase 25 / LNX-04): the former TestNonWindows_Unsupported was removed.
// The !windows stubs NO LONGER return ErrUnsupported — they are now real CLI
// stdin prompts (dialog_other.go). Their behavior (trim / empty / EOF-cancel /
// path expansion) is covered by dialog_other_test.go. ErrUnsupported is retained
// as a distinct sentinel for back-compat (asserted above).
