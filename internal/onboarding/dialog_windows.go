//go:build windows

package onboarding

// dialog_windows.go implements the native Win32 "paste your guild code" input
// dialog (CONTEXT D-3, FORK 1 Option A). It builds an in-memory DLGTEMPLATE (a
// static-label prompt + a single-line EDIT control + OK/Cancel buttons) and runs
// it modally via user32!DialogBoxIndirectParamW with a Go dialog-proc callback.
// This is pure Win32 over golang.org/x/sys/windows — ZERO network surface (no
// TCP listener, no HTTP handler, no loopback server): the localhost listener the
// Phase 13 deletion removes is NOT reintroduced here (RESEARCH Pitfall 3).
//
// Why DialogBoxIndirectParamW (an in-memory template) over a hand-rolled
// CreateWindowEx message loop: the dialog manager gives us correct keyboard
// behavior for free — Tab moves between controls, Enter activates the default
// (IDOK), Esc maps to IDCANCEL — and a self-contained modal pump, with no window
// class to register/unregister and no manual GetMessage/DispatchMessage loop to
// maintain. The cost is constructing the template's binary layout by hand, which
// is well-documented below.
//
// The Win32 entry points are resolved lazily via windows.NewLazySystemDLL
// (mirrors cmd/squirebot/console_windows.go) — NewLazySystemDLL forces the
// system32 search path, mitigating DLL-preload attacks.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"strings"
	"sync"
	"unsafe"

	"github.com/sqweek/dialog"
	"golang.org/x/sys/windows"
)

// errDialogCreate is returned when DialogBoxIndirectParamW fails to create the
// modal dialog (it returns -1). Distinct from ErrCancelled (a user dismissal).
var errDialogCreate = errors.New("onboarding: failed to create input dialog")

// user32 + the Win32 procs the dialog needs, resolved at first use from
// system32 (NewLazySystemDLL = DLL-preload-safe; see console_windows.go).
var (
	user32 = windows.NewLazySystemDLL("user32.dll")

	procDialogBoxIndirectParamW = user32.NewProc("DialogBoxIndirectParamW")
	procEndDialog               = user32.NewProc("EndDialog")
	procGetDlgItemTextW         = user32.NewProc("GetDlgItemTextW")
	procSetDlgItemTextW         = user32.NewProc("SetDlgItemTextW")
	procGetDlgItem              = user32.NewProc("GetDlgItem")
	procSetFocus                = user32.NewProc("SetFocus")
)

// Win32 constants (subset; values from WinUser.h).
const (
	// Window styles.
	wsChild        = 0x40000000
	wsVisible      = 0x10000000
	wsBorder       = 0x00800000
	wsTabStop      = 0x00010000
	wsGroup        = 0x00020000
	wsPopup        = 0x80000000
	wsCaption      = 0x00C00000 // WS_BORDER | WS_DLGFRAME
	wsSysMenu      = 0x00080000
	esAutoHScroll  = 0x00000080
	bsDefPushBtn   = 0x00000001
	dsModalFrame   = 0x00000080
	dsSetFont      = 0x00000040
	dsCenter       = 0x00000800
	ssLeft         = 0x00000000

	// Dialog/control messages.
	wmInitDialog = 0x0110
	wmCommand    = 0x0111
	wmClose      = 0x0010

	// Standard control IDs.
	idOK     = 1
	idCancel = 2

	// Predefined control-class atoms (the high WORD 0xFFFF marker + the atom).
	atomButton = 0x0080
	atomEdit   = 0x0081
	atomStatic = 0x0082

	// Our control IDs.
	idEdit   = 1000
	idPrompt = 1001

	maxCodeLen = 512 // generous cap on the entered code length (chars).
)

// dialogState carries the per-invocation prompt text in and the entered code +
// outcome out.
type dialogState struct {
	prompt   string
	code     string
	accepted bool
}

// dialogRegistry pins live dialogStates so they stay GC-reachable for the dialog's
// lifetime, and bridges the two lookups the DLGPROC needs WITHOUT ever converting
// a uintptr back into a Go pointer (which `go vet` flags and the GC does not track
// across the syscall/callback boundary — a Go pointer passed as dwInitParam and
// reconstructed via unsafe.Pointer(lParam) could be moved/collected). Instead we
// pass a small integer TOKEN as dwInitParam, look the state up by that token on
// WM_INITDIALOG, then re-key it by hwnd for the rest of the dialog. The mutex
// guards both maps (onboarding dialogs are serial, but this keeps the package
// correct under serial reuse from any goroutine).
var dialogRegistry struct {
	mu        sync.Mutex
	nextTok   uintptr
	byToken   map[uintptr]*dialogState
	byHwnd    map[uintptr]*dialogState
}

func init() {
	dialogRegistry.byToken = map[uintptr]*dialogState{}
	dialogRegistry.byHwnd = map[uintptr]*dialogState{}
}

func registerState(st *dialogState) uintptr {
	dialogRegistry.mu.Lock()
	defer dialogRegistry.mu.Unlock()
	dialogRegistry.nextTok++
	tok := dialogRegistry.nextTok
	dialogRegistry.byToken[tok] = st
	return tok
}

// claimByToken moves the state from the token map to the hwnd map (called once on
// WM_INITDIALOG). Returns nil if the token is unknown.
func claimByToken(tok, hwnd uintptr) *dialogState {
	dialogRegistry.mu.Lock()
	defer dialogRegistry.mu.Unlock()
	st := dialogRegistry.byToken[tok]
	if st == nil {
		return nil
	}
	delete(dialogRegistry.byToken, tok)
	dialogRegistry.byHwnd[hwnd] = st
	return st
}

func stateForHwnd(hwnd uintptr) *dialogState {
	dialogRegistry.mu.Lock()
	defer dialogRegistry.mu.Unlock()
	return dialogRegistry.byHwnd[hwnd]
}

func releaseHwnd(hwnd uintptr) {
	dialogRegistry.mu.Lock()
	defer dialogRegistry.mu.Unlock()
	delete(dialogRegistry.byHwnd, hwnd)
}

// PromptGuildCode opens a native modal text-input dialog titled title with the
// label prompt and a single-line edit box, plus OK/Cancel buttons. It returns the
// trimmed entered string on OK, or ErrCancelled on Cancel/Esc/close. The returned
// error is non-nil only on a Win32 failure (e.g. the dialog could not be created)
// or ErrCancelled.
func PromptGuildCode(title, prompt string) (string, error) {
	tmpl, err := buildInputDialogTemplate(title)
	if err != nil {
		return "", err
	}

	st := &dialogState{prompt: prompt}
	tok := registerState(st)

	cb := windows.NewCallback(dialogProc)

	// hwndParent = 0 (no owner — the watcher is a tray app with no main window);
	// hInstance = 0 (the template is self-contained); dwInitParam carries the
	// integer TOKEN (NOT a Go pointer) which arrives as the WM_INITDIALOG lParam.
	ret, _, _ := procDialogBoxIndirectParamW.Call(
		0,                                 // hInstance
		uintptr(unsafe.Pointer(&tmpl[0])), // lpTemplate (in-memory DLGTEMPLATE)
		0,                                 // hWndParent
		cb,                                // lpDialogFunc
		tok,                               // dwInitParam -> WM_INITDIALOG lParam (a token)
	)
	// DialogBoxIndirectParamW returns the nResult passed to EndDialog. We use
	// idOK / idCancel as the result; -1 (0xFFFFFFFF) indicates a creation failure.
	if int(ret) == -1 {
		return "", errDialogCreate
	}
	if !st.accepted {
		return "", ErrCancelled
	}
	code := strings.TrimSpace(st.code)
	if code == "" {
		// An empty OK is treated as a cancel (nothing useful to validate).
		return "", ErrCancelled
	}
	return code, nil
}

// dialogProc is the DLGPROC callback. It returns TRUE (1) if it handled the
// message, FALSE (0) to let the dialog manager handle it. On WM_INITDIALOG it
// claims the dialogState by the integer token passed as lParam (re-keying it by
// hwnd); thereafter it looks the state up by hwnd. No uintptr is ever converted
// back to a Go pointer.
func dialogProc(hwnd uintptr, msg uintptr, wParam uintptr, lParam uintptr) uintptr {
	switch msg {
	case wmInitDialog:
		st := claimByToken(lParam, hwnd) // lParam is the token, not a pointer
		if st != nil {
			setDlgItemText(hwnd, idPrompt, st.prompt)
		}
		focusDlgItem(hwnd, idEdit)
		return 0 // we set focus ourselves -> return FALSE per DLGPROC contract.

	case wmCommand:
		id := int(wParam & 0xFFFF) // LOWORD(wParam) = control ID
		switch id {
		case idOK:
			if st := stateForHwnd(hwnd); st != nil {
				st.code = getDlgItemText(hwnd, idEdit)
				st.accepted = true
			}
			endDialog(hwnd, idOK)
			return 1
		case idCancel:
			if st := stateForHwnd(hwnd); st != nil {
				st.accepted = false
			}
			endDialog(hwnd, idCancel)
			return 1
		}

	case wmClose:
		if st := stateForHwnd(hwnd); st != nil {
			st.accepted = false
		}
		endDialog(hwnd, idCancel)
		return 1
	}
	return 0
}

func endDialog(hwnd uintptr, result int) {
	releaseHwnd(hwnd)
	procEndDialog.Call(hwnd, uintptr(result))
}

func setDlgItemText(hwnd uintptr, id int, text string) {
	p, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	procSetDlgItemTextW.Call(hwnd, uintptr(id), uintptr(unsafe.Pointer(p)))
}

func focusDlgItem(hwnd uintptr, id int) {
	h, _, _ := procGetDlgItem.Call(hwnd, uintptr(id))
	if h != 0 {
		procSetFocus.Call(h)
	}
}

func getDlgItemText(hwnd uintptr, id int) string {
	buf := make([]uint16, maxCodeLen+1)
	n, _, _ := procGetDlgItemTextW.Call(
		hwnd,
		uintptr(id),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	return windows.UTF16ToString(buf[:n])
}

// --- DLGTEMPLATE construction -------------------------------------------------

// buildInputDialogTemplate constructs an in-memory DLGTEMPLATE (the "EX"-less
// classic form) describing:
//
//	the dialog itself  : WS_POPUP|WS_CAPTION|WS_SYSMENU|DS_MODALFRAME|DS_CENTER, titled `title`
//	a STATIC label     : id idPrompt (the prompt text is set at WM_INITDIALOG)
//	an EDIT box        : id idEdit, single-line, WS_BORDER|WS_TABSTOP|ES_AUTOHSCROLL
//	an OK button       : id idOK (IDOK), BS_DEFPUSHBUTTON|WS_TABSTOP (the default)
//	a Cancel button    : id idCancel (IDCANCEL), WS_TABSTOP
//
// The binary layout follows MSDN's DLGTEMPLATE / DLGITEMTEMPLATE spec: a
// DLGTEMPLATE header, then WORD-sized menu (0) + class (0) + the title as a
// NUL-terminated UTF-16 string; then, for each control, the block is aligned to a
// DWORD boundary, a DLGITEMTEMPLATE header, the control class (0xFFFF + atom), the
// control title (0 here — set at runtime), and a 0 creation-data WORD. Coordinates
// are in dialog units.
func buildInputDialogTemplate(title string) ([]byte, error) {
	titleU16, err := windows.UTF16FromString(title)
	if err != nil {
		return nil, err
	}

	var b bytes.Buffer

	// DLGTEMPLATE { DWORD style; DWORD dwExtendedStyle; WORD cdit;
	//               short x, y, cx, cy; } + menu/class/title (sz_Or_Ord arrays).
	style := uint32(wsPopup | wsCaption | wsSysMenu | dsModalFrame | dsCenter)
	writeU32(&b, style)
	writeU32(&b, 0) // dwExtendedStyle
	writeU16(&b, 4) // cdit = 4 controls (label, edit, OK, Cancel)
	writeI16(&b, 0) // x
	writeI16(&b, 0) // y
	writeI16(&b, 220) // cx (dialog units)
	writeI16(&b, 90)  // cy
	writeU16(&b, 0)   // menu: none (0x0000)
	writeU16(&b, 0)   // windowClass: none -> default dialog class
	writeU16Slice(&b, titleU16) // title (NUL-terminated UTF-16)

	// Control 1: STATIC prompt label.
	writeControl(&b, control{
		style: uint32(wsChild | wsVisible | ssLeft),
		x:     10, y:  8, cx: 200, cy: 20,
		id:    idPrompt, atom: atomStatic,
	})

	// Control 2: EDIT box.
	writeControl(&b, control{
		style: uint32(wsChild | wsVisible | wsBorder | wsTabStop | wsGroup | esAutoHScroll),
		x:     10, y: 34, cx: 200, cy: 14,
		id:    idEdit, atom: atomEdit,
	})

	// Control 3: OK button (default).
	writeControl(&b, control{
		style: uint32(wsChild | wsVisible | wsTabStop | bsDefPushBtn),
		x:     58, y: 60, cx: 44, cy: 16,
		id:    idOK, atom: atomButton, title: "OK",
	})

	// Control 4: Cancel button.
	writeControl(&b, control{
		style: uint32(wsChild | wsVisible | wsTabStop),
		x:     116, y: 60, cx: 44, cy: 16,
		id:    idCancel, atom: atomButton, title: "Cancel",
	})

	return b.Bytes(), nil
}

type control struct {
	style          uint32
	x, y, cx, cy   int16
	id             int
	atom           uint16 // predefined class atom (button/edit/static)
	title          string // optional caption baked into the template
}

// writeControl appends a DWORD-aligned DLGITEMTEMPLATE for c.
func writeControl(b *bytes.Buffer, c control) {
	alignDWORD(b)
	// DLGITEMTEMPLATE { DWORD style; DWORD dwExtendedStyle;
	//                   short x, y, cx, cy; WORD id; } + class + title + creationData.
	writeU32(b, c.style)
	writeU32(b, 0) // dwExtendedStyle
	writeI16(b, c.x)
	writeI16(b, c.y)
	writeI16(b, c.cx)
	writeI16(b, c.cy)
	writeU16(b, uint16(c.id))
	// Class: 0xFFFF marker WORD followed by the predefined atom WORD.
	writeU16(b, 0xFFFF)
	writeU16(b, c.atom)
	// Title: a NUL-terminated UTF-16 string (empty -> just the NUL) baked in.
	if c.title == "" {
		writeU16(b, 0)
	} else {
		u, _ := windows.UTF16FromString(c.title)
		writeU16Slice(b, u)
	}
	// Creation data: none.
	writeU16(b, 0)
}

func writeU16(b *bytes.Buffer, v uint16) { _ = binary.Write(b, binary.LittleEndian, v) }
func writeI16(b *bytes.Buffer, v int16)  { _ = binary.Write(b, binary.LittleEndian, v) }
func writeU32(b *bytes.Buffer, v uint32) { _ = binary.Write(b, binary.LittleEndian, v) }

func writeU16Slice(b *bytes.Buffer, vs []uint16) {
	for _, v := range vs {
		writeU16(b, v)
	}
}

// alignDWORD pads the buffer with zero bytes up to the next 4-byte boundary (each
// DLGITEMTEMPLATE must start DWORD-aligned per MSDN).
func alignDWORD(b *bytes.Buffer) {
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
}

// --- EQ-folder picker (relocated from internal/wizard/folderpicker_dialog.go) -

// PickEQFolder opens the native folder-selection dialog (sqweek/dialog) so the
// guildie chooses their EverQuest install directory during onboarding. This is
// the verbatim relocation of internal/wizard/folderpicker_dialog.go::
// defaultPickFolder; it survives the wizard deletion because the EQ-folder step
// is still needed (alongside the guild code) on first run. sqweek returns
// dialog.ErrCancelled on user-cancel, which we map to onboarding.ErrCancelled.
//
// The CALLER (Plan 03) runs eqfind.ValidateFolder on the returned path and
// re-prompts with the verbatim "doesn't look like an EverQuest install" message;
// this function just returns the chosen path.
func PickEQFolder(title string) (string, error) {
	dir, err := dialog.Directory().Title(title).Browse()
	if err != nil {
		if err == dialog.Cancelled {
			return "", ErrCancelled
		}
		return "", err
	}
	return dir, nil
}
