//go:build windows

package auth

import (
	"fmt"
	"testing"
	"time"

	"github.com/danieljoos/wincred"
)

// uniqueEmail returns a per-test wincred target name that cannot
// collide with a real user's credentials. example.invalid is an
// IETF-reserved TLD so it can never be a Google account.
func uniqueEmail(name string) string {
	return fmt.Sprintf("squirebot-test-%s-%d@example.invalid", name, time.Now().UnixNano())
}

func TestStoreAndReadRoundTrip(t *testing.T) {
	email := uniqueEmail("roundtrip")
	t.Cleanup(func() { _ = DeleteToken(email) })

	want := StoredToken{
		RefreshToken: "1//0xfakerefreshfor-tests-only",
		Email:        email,
		ClientID:     "test-client-id.apps.googleusercontent.com",
	}
	if err := StoreToken(email, want); err != nil {
		t.Fatalf("StoreToken: %v", err)
	}

	got, err := ReadToken(email)
	if err != nil {
		t.Fatalf("ReadToken: %v", err)
	}
	if got != want {
		t.Errorf("round-trip mismatch:\n  got=%+v\n want=%+v", got, want)
	}
}

func TestReadMissingReturnsError(t *testing.T) {
	email := uniqueEmail("missing")
	// Do NOT store anything. ReadToken should return wincred's not-found error.
	_, err := ReadToken(email)
	if err == nil {
		t.Fatalf("ReadToken on missing credential: want error, got nil")
	}
}

func TestDeleteRemovesCredential(t *testing.T) {
	email := uniqueEmail("delete")

	if err := StoreToken(email, StoredToken{RefreshToken: "x", Email: email, ClientID: "c"}); err != nil {
		t.Fatalf("StoreToken: %v", err)
	}
	if err := DeleteToken(email); err != nil {
		t.Fatalf("DeleteToken: %v", err)
	}
	if _, err := ReadToken(email); err == nil {
		t.Errorf("ReadToken after Delete: want error, got nil")
	}
}

// TestStoreUsesJSONBlob asserts the CredentialBlob is valid JSON
// matching StoredToken — not a string concat or some other format.
// This guards against a future "optimisation" that switches to a
// custom encoding and silently breaks ReadToken across versions.
func TestStoreUsesJSONBlob(t *testing.T) {
	email := uniqueEmail("blob-shape")
	t.Cleanup(func() { _ = DeleteToken(email) })

	st := StoredToken{RefreshToken: "rt", Email: email, ClientID: "cid"}
	if err := StoreToken(email, st); err != nil {
		t.Fatalf("StoreToken: %v", err)
	}

	cred, err := wincred.GetGenericCredential(CredPrefix + email)
	if err != nil {
		t.Fatalf("GetGenericCredential: %v", err)
	}
	// The first byte of a JSON object is '{'. A string-concat would
	// almost certainly start with another character.
	if len(cred.CredentialBlob) == 0 || cred.CredentialBlob[0] != '{' {
		t.Errorf("CredentialBlob does not look like JSON: first bytes = %q",
			string(cred.CredentialBlob[:min(40, len(cred.CredentialBlob))]))
	}
	if cred.Persist != wincred.PersistLocalMachine {
		t.Errorf("Persist = %v, want PersistLocalMachine", cred.Persist)
	}
}

// TestCredPrefixLiteral is a hard pin on the AUTH-04 target-name
// pattern. Any future renaming MUST update REQUIREMENTS.md AUTH-04
// AND every stored credential migrates — break this test and you're
// breaking the contract on every existing guildie's machine.
func TestCredPrefixLiteral(t *testing.T) {
	if CredPrefix != "SquireBot:" {
		t.Fatalf("CredPrefix = %q, want %q", CredPrefix, "SquireBot:")
	}
}

