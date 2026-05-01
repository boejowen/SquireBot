package wizard

// defaultPickFolder is the production binding for the wizard's native
// folder picker. sqweek/dialog opens the standard Windows / macOS / Linux
// folder dialog. On user-cancel sqweek returns dialog.ErrCancelled which
// we surface as a non-nil error; the wizard handler then replies 204.
//
// This file is compiled on every platform (sqweek/dialog has cross-
// platform stubs). Plan 01-01 pinned sqweek/dialog precisely so this
// import is uncontroversial here.

import (
	"github.com/sqweek/dialog"
)

func defaultPickFolder(title string) (string, error) {
	return dialog.Directory().Title(title).Browse()
}
