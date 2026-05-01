package auth

import (
	"encoding/json"

	"github.com/danieljoos/wincred"
)

// CredPrefix is the wincred target-name prefix for SquireBot's stored
// refresh tokens. Per AUTH-04 the canonical target name is the literal
// concatenation `SquireBot:<google-email>`. The colon-delimited form is
// the standard wincred convention and groups all SquireBot credentials
// together in `cmdkey /list`.
const CredPrefix = "SquireBot:"

// StoredToken is the JSON payload written into the wincred
// CredentialBlob. Persisting the email + client_id alongside the
// refresh token lets the watcher detect on read whether the credential
// was issued for the current OAuth client (sanity check after a Cloud
// project migration). The struct is JSON-marshalled for forward-compat:
// adding fields later does not invalidate existing entries.
type StoredToken struct {
	RefreshToken string `json:"refresh_token"`
	Email        string `json:"email"`
	ClientID     string `json:"client_id"`
}

// StoreToken writes st under target name `SquireBot:<email>` with
// PersistLocalMachine — the credential survives reboots and is
// encrypted at rest by DPAPI keyed to the user profile. AUTH-04 / T-03-08.
func StoreToken(email string, st StoredToken) error {
	blob, err := json.Marshal(st)
	if err != nil {
		return err
	}
	cred := wincred.NewGenericCredential(CredPrefix + email)
	cred.CredentialBlob = blob
	cred.Persist = wincred.PersistLocalMachine
	return cred.Write()
}

// ReadToken returns the StoredToken previously written for email.
// Returns the underlying wincred not-found error if the credential is
// absent (callers treat this as "first run, re-OAuth needed" — the
// same code path as a fresh install per RESEARCH.md §4.7).
func ReadToken(email string) (StoredToken, error) {
	cred, err := wincred.GetGenericCredential(CredPrefix + email)
	if err != nil {
		return StoredToken{}, err
	}
	var st StoredToken
	if err := json.Unmarshal(cred.CredentialBlob, &st); err != nil {
		return StoredToken{}, err
	}
	return st, nil
}

// DeleteToken removes the wincred entry for email. Used by the
// "Sign Out" tray menu (Phase 2) and by the re-OAuth path when a
// refresh fails with invalid_grant (AUTH-05, also Phase 2).
func DeleteToken(email string) error {
	cred, err := wincred.GetGenericCredential(CredPrefix + email)
	if err != nil {
		return err
	}
	return cred.Delete()
}
