# SmartScreen Walkthrough — Installing SquireBot

SquireBot ships unsigned in Phase 2. Windows Defender SmartScreen will show one of three prompts depending on your browser when you double-click `SquireBot-Setup-<version>.exe`. All three lead to the same install in **under 30 seconds**.

> **Why unsigned?** Code-signing certificates no longer grant instant SmartScreen reputation as of March 2024 (Microsoft removed EV's reputation perk; in August 2024 they removed all EV Code Signing OIDs from existing roots in the Trusted Root Program). A 12-user app would never accumulate enough downloads to clear the reputation curve, regardless of cert type. We're applying for free SignPath Foundation OSS code signing in parallel; until that approves, the walkthrough below is the recommended path. See [signpath-application.md](signpath-application.md) for status.

The technical reason this prompt fires: your browser tags the downloaded `.exe` with the **Mark-of-the-Web (MOTW)** Alternate Data Stream. Windows Defender SmartScreen checks MOTW-tagged binaries against its reputation database on first execution, and an "Unknown publisher" + low download count = full blue panel. Removing MOTW (right-click `.exe` → Properties → "Unblock" checkbox) suppresses the prompt but is NOT recommended — you lose the safety net for actually-malicious files you may download in the future.

## Path A: Microsoft Edge (default Windows browser)

1. Click `SquireBot-Setup-<version>.exe` from your Edge download bar / downloads page. Edge may show a small "..." menu next to the file with options including **Keep**. If asked, click **Keep**. (~5 seconds.)
2. Double-click the saved `SquireBot-Setup-<version>.exe`.
3. Windows Defender SmartScreen shows a blue panel titled **"Microsoft Defender SmartScreen prevented an unrecognized app from starting"** (older Win11 builds say **"Windows protected your PC"** — same dialog, same buttons).
4. Click the small **"More info"** link near the bottom-left of the dialog. The panel expands to show:
   - "App: SquireBot-Setup-`<version>`.exe"
   - "Publisher: Unknown publisher"
   - A new button: **"Run anyway"**.
5. Click **"Run anyway"**.
6. The NSIS installer wizard opens. Total elapsed: **~25 seconds** for a first-time user.

*Screenshot placeholder: edge-smartscreen-1.png (the blue panel with "More info" highlighted)*
*Screenshot placeholder: edge-smartscreen-2.png (after clicking More info — "Run anyway" highlighted)*

## Path B: Google Chrome

1. After download, Chrome's bottom bar (or downloads tray) may show a **"Keep / Discard"** prompt for the .exe. Click **"Keep"**. (~5 seconds.)
   - Some recent Chrome versions show a fuller "This file is not commonly downloaded and may be dangerous" — click **"Keep dangerous file"** → **"Keep anyway"**. The wording shifts every few releases; the Keep-equivalent button is always present.
2. Double-click `SquireBot-Setup-<version>.exe`.
3. Windows Defender SmartScreen shows the same blue panel as Edge: **"Microsoft Defender SmartScreen prevented..."** / **"Windows protected your PC"**.
4. Click **"More info"** → **"Run anyway"**, identical to Path A steps 4–5.

*Screenshot placeholder: chrome-keep-discard.png (the bottom-bar prompt)*

## Path C: Mozilla Firefox

Firefox does NOT tag downloads with the Windows Mark-of-the-Web (MOTW) by default, so SmartScreen often does not engage at all on Firefox-downloaded installers. You should be able to double-click and run with no Windows-level reputation prompt.

However, Defender (the runtime AV layer) may still scan the binary at execution and flag it. If you see a **Windows Defender SmartScreen** prompt at any point, follow the same **"More info" → "Run anyway"** sequence from Path A.

> **Caveat:** if your guild standardizes on Firefox + an aggressive security suite (Norton, McAfee, ESET, Bitdefender), ask your guild leader before clicking through any prompts. Some AV products quarantine unsigned binaries silently with no user-visible prompt; see the next section for recovery.

## What if Windows Defender quarantines SquireBot AFTER install?

Sometimes Defender catches an unknown-publisher binary post-install (during a real-time scan or a scheduled scan) and silently moves it to quarantine. Symptoms:

- The tray icon disappears.
- `%LOCALAPPDATA%\Programs\SquireBot\squirebot.exe` no longer exists on disk.
- The autostart entry under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run\SquireBot` still exists but points at a missing file.

Recovery:

1. Open **Windows Security** (Start menu → "Windows Security") → **Virus & threat protection** → **Protection history**.
2. Find the most recent quarantine event for `squirebot.exe`.
3. Click **Actions** → **Restore**, then under "Allowed threats" add **Allow on device**.
4. SquireBot reappears at next logon (autostart fires the restored binary).
5. If Defender quarantines again on the next launch, file a false-positive report at <https://www.microsoft.com/en-us/wdsi/filesubmission> so SquireBot's hash gets whitelisted globally for future guildies. Reference SignPath Foundation's OSS sponsorship application (see [signpath-application.md](signpath-application.md)) in the report.

If your guild uses a third-party AV (not Defender) that silently quarantined the binary, the recovery flow is product-specific — check your AV's "Quarantine" / "Threat history" UI for a Restore option.

## Why this is "good enough" at our scale

| Bar                                                | Met by SquireBot                                                                          |
| -------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Install completes in <30 seconds without admin rights | Yes (NSIS per-user, no UAC; SmartScreen "Run anyway" in 5s)                            |
| Documented for non-technical users                 | Yes (this doc + screenshots + browser-specific paths for Edge / Chrome / Firefox)         |
| False-positive recovery procedure exists           | Yes (Defender quarantine restore steps above)                                             |
| Path to signed binary if needed later              | Yes — [signpath-application.md](signpath-application.md) tracks the OSS application      |

For a 12-guildie audience installing once and forgetting, this is the right ergonomics-vs-cost tradeoff. See [.planning/PROJECT.md](../.planning/PROJECT.md) for the project context and [build-and-install.md](build-and-install.md) for the local build runbook.

## macOS / Linux

Out of scope. SquireBot is Windows-only because EverQuest's `/outputfile inventory` writes to a fixed Windows path on the EverQuest install directory; the watcher's value is its location next to that install. P1999 itself is a Windows EverQuest emulator client.

---

*Last updated: 2026-05 (Phase 2 Plan 02-09). [.planning/research/PITFALLS.md Pitfall #2](../.planning/research/PITFALLS.md) contains outdated guidance on EV-cert reputation; defer to this doc and to [02-RESEARCH.md §1 Code-Signing Recommendation Matrix](../.planning/phases/02-watcher-robustness-schema-lock/02-RESEARCH.md).*
