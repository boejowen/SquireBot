// Package credstore is the watcher-side DPAPI store for the v2.0 guild code
// (WATCH-10 / CONTEXT D-6). It is the salvaged survivor of internal/auth/store.go:
// the wincred mechanics (NewGenericCredential / PersistLocalMachine / Write /
// GetGenericCredential / Delete) are UNCHANGED; only the blob and the target name
// are reshaped — the v1 JSON {refresh_token, email, client_id} blob keyed by
// SquireBot:<email> becomes the raw guild code under a single fixed target.
//
// SECURITY (carries over verbatim from the v1 AUTH-04 rule): the guild code lives
// ONLY in Windows Credential Manager (DPAPI-encrypted at rest, keyed to the user
// profile). It is NEVER written to config.json — internal/config's SECURITY
// comment documents "NEVER add a secret field"; the same prohibition applies
// here. It is NEVER logged (V7).
//
// The plaintext code is stored (NOT a hash): unlike the SERVER, which persists
// only sha256(code), the watcher must present the PLAINTEXT code as the Bearer
// value on every POST to /api/v1/ingest. DPAPI is the at-rest protection; the
// plaintext is decryptable only by the user's own profile.
//
// This file is intentionally NOT build-tagged, matching the survivor
// (internal/auth/store.go was un-tagged): github.com/danieljoos/wincred compiles
// on every platform, and the dev box is Windows (CONTEXT code_context), so the
// round-trip test exercises real DPAPI there while still building everywhere.
package credstore

import (
	"github.com/danieljoos/wincred"
)

// credTarget is the single fixed Windows Credential Manager target name for the
// guild code (D-6 / Assumption A4): exactly one credential per machine, with NO
// email/identity component in the key (v2 identity is derived server-side from
// the code, so there is no email to key on). The "SquireBot:" prefix groups it
// with other SquireBot credentials in `cmdkey /list` (the v1 convention).
const credTarget = "SquireBot:guild-code"

// Store writes the guild code under credTarget with PersistLocalMachine — the
// credential survives reboots and is DPAPI-encrypted at rest, keyed to the user
// profile. The PLAINTEXT code is stored because the watcher must present it as
// the Bearer value on every ingest POST (the server stores only the hash).
//
// SECURITY: the code is NEVER written to config.json (the AUTH-04 rule) and
// NEVER logged (V7).
func Store(code string) error {
	cred := wincred.NewGenericCredential(credTarget)
	cred.CredentialBlob = []byte(code)
	cred.Persist = wincred.PersistLocalMachine
	return cred.Write()
}

// Read returns the stored guild code. A not-found error (the underlying wincred
// error when no credential exists) signals "first run / needs onboarding" — the
// caller (Plan 03's onboarding flow) treats it as the trigger to prompt the
// guildie for their code.
func Read() (string, error) {
	cred, err := wincred.GetGenericCredential(credTarget)
	if err != nil {
		return "", err
	}
	return string(cred.CredentialBlob), nil
}

// Delete removes the stored guild code. It returns the not-found error if no
// credential is present; callers that want idempotent teardown (e.g.
// --uninstall-wipe-credentials, or a re-onboard) ignore not-found.
func Delete() error {
	cred, err := wincred.GetGenericCredential(credTarget)
	if err != nil {
		return err
	}
	return cred.Delete()
}
