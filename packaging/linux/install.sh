#!/bin/sh
# SquireBot watcher — Linux installer (Phase 25 / LNX-05, D-06).
#
# Installs the static squirebot binary to ~/.local/bin, drops the systemd
# USER unit to ~/.config/systemd/user, runs first-time onboarding if the
# watcher is not yet configured, and enables + starts the autostart service.
#
# Idempotent: re-running overwrites the binary + unit and re-enables the
# (already-enabled) service safely.
#
# Usage:
#   ./install.sh            # default: starts on desktop login
#   ./install.sh --linger   # also enable lingering (headless/SSH-only boxes
#                           # that must run without a graphical login session)
#
# Run from the extracted tarball directory (it must contain `squirebot` and
# `squirebot.service` alongside this script). Do NOT run as root — this is a
# per-user install.

set -eu

# --- argument parsing -------------------------------------------------------
ENABLE_LINGER=0
for arg in "$@"; do
	case "$arg" in
		--linger) ENABLE_LINGER=1 ;;
		-h|--help)
			echo "Usage: $0 [--linger]"
			echo "  --linger  enable loginctl lingering (autostart at boot without a"
			echo "            graphical login; for headless/SSH-only boxes)"
			exit 0
			;;
		*)
			echo "error: unknown argument: $arg" >&2
			echo "Usage: $0 [--linger]" >&2
			exit 2
			;;
	esac
done

# --- locate the tarball payload --------------------------------------------
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
BIN_SRC="$SCRIPT_DIR/squirebot"
UNIT_SRC="$SCRIPT_DIR/squirebot.service"

if [ ! -f "$BIN_SRC" ]; then
	echo "error: squirebot binary not found next to install.sh ($BIN_SRC)" >&2
	echo "       run install.sh from inside the extracted tarball directory." >&2
	exit 1
fi
if [ ! -f "$UNIT_SRC" ]; then
	echo "error: squirebot.service not found next to install.sh ($UNIT_SRC)" >&2
	exit 1
fi

BIN_DST="$HOME/.local/bin/squirebot"
UNIT_DST="$HOME/.config/systemd/user/squirebot.service"

# --- install the binary -----------------------------------------------------
echo "==> Installing binary to $BIN_DST"
install -Dm755 "$BIN_SRC" "$BIN_DST"

# --- install the systemd user unit -----------------------------------------
echo "==> Installing systemd user unit to $UNIT_DST"
install -Dm644 "$UNIT_SRC" "$UNIT_DST"

echo "==> Reloading the systemd user manager"
systemctl --user daemon-reload

# --- first-run onboarding (only if unconfigured) ---------------------------
# `squirebot --status` exits non-zero when the guild code / EQ folder are not
# yet configured; in that case run the interactive CLI setup. If already
# configured, this is a no-op so re-running install.sh never re-prompts.
echo "==> Checking configuration"
if "$BIN_DST" --status >/dev/null 2>&1; then
	echo "    already configured — skipping setup."
else
	echo "    not configured — launching first-time setup (--setup)."
	"$BIN_DST" --setup
fi

# --- enable + start the autostart service ----------------------------------
echo "==> Enabling and starting the squirebot user service"
systemctl --user enable --now squirebot.service

# --- optional: linger (opt-in, default OFF) --------------------------------
if [ "$ENABLE_LINGER" -eq 1 ]; then
	echo "==> Enabling lingering for $USER (loginctl enable-linger)"
	echo "    the watcher will now start at boot and survive logout."
	loginctl enable-linger "$USER"
fi

echo ""
echo "Installed. SquireBot is running as a systemd user service."
echo "  status:  systemctl --user status squirebot   (or: squirebot --status)"
echo "  logs:    journalctl --user -u squirebot -f"
if [ "$ENABLE_LINGER" -ne 1 ]; then
	echo "  note:    starts on your next desktop login. For a headless/SSH-only"
	echo "           box that must run without logging in, re-run: ./install.sh --linger"
fi
