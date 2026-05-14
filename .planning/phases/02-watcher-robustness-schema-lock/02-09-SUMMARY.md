---
phase: 02-watcher-robustness-schema-lock
plan: 09
subsystem: code-signing-walkthrough
tags: [docs, smartscreen, signpath, oss, ci, code-signing, inst-05, walkthrough]
requires:
  - .github/workflows/release.yml (Plan 02-08 LINEAR workflow with explicit makensis + SHA-256 steps)
  - .planning/phases/02-watcher-robustness-schema-lock/02-RESEARCH.md (§1 Code-Signing Recommendation Matrix)
  - .planning/phases/02-watcher-robustness-schema-lock/02-CONTEXT.md (Code Signing Q1 locked decisions)
  - docs/build-and-install.md (cross-link target for guildie install runbook)
provides:
  - docs/smartscreen-walkthrough.md (browser-specific Edge / Chrome / Firefox install paths + Defender quarantine recovery)
  - docs/signpath-application.md (SignPath Foundation OSS application tracker + copy-paste-ready application package)
  - README.md (guildie-facing rewrite: install + tray menu + 'tray turned red' recovery + signing rationale)
  - .github/workflows/release.yml (TODO(SignPath OSS) scaffold block at correct insertion point)
affects:
  - Future SignPath enablement (single-step workflow change at the documented insertion point; no .goreleaser.yaml change)
  - Phase 5 polish (DOC-03 owns onboarding screenshots/video; this plan ships placeholder text)
  - INST-05 (now MET via documented walkthrough <30s; OR-clause satisfied)
tech-stack:
  added:
    - SignPath Foundation application materials (free OSS sponsorship; cloud-HSM-backed signing when approved)
  patterns:
    - Defer-to-user pattern for unautomatable web-form submissions: write a complete copy-paste application package + tracker doc, mark task as DEFERRED in SUMMARY
    - TODO comment scaffolds in CI workflows at exact future-insertion points (preserves architectural intent across multi-phase work)
    - Documentation-first INST-05 satisfaction (walkthrough is the deliverable, not the cert)
key-files:
  created:
    - docs/smartscreen-walkthrough.md
    - docs/signpath-application.md
    - .planning/phases/02-watcher-robustness-schema-lock/02-09-SUMMARY.md
  modified:
    - README.md (Phase-1-skeleton stub → guildie-facing v1 onboarding doc)
    - .github/workflows/release.yml (added TODO(SignPath OSS) scaffold block between makensis and SHA-256)
decisions:
  - SmartScreen walkthrough at docs/smartscreen-walkthrough.md is THE INST-05 deliverable. Browser-specific paths (Edge default, Chrome 'Keep / Discard' variant, Firefox no-MOTW special case) cover Pitfall E in 02-RESEARCH.md.
  - SignPath GitHub Action will plug in BETWEEN 'Build NSIS installer' and 'Compute SHA-256 sums' in .github/workflows/release.yml -- NOT in .goreleaser.yaml. Plan 02-08 deliberately omitted the goreleaser signs: block because CI does not invoke goreleaser; this plan reinforces that.
  - SignPath application is DEFERRED to user submission (cannot be filed by an executor: real human + GitHub MFA + form completion required). Tracker doc carries a complete copy-paste application package to make the user's submission step trivial.
  - LICENSE file is MISSING from repo root and must be added before the SignPath application is filed (SignPath requires OSI-approved license). Documented in tracker; not auto-added because picking a license is a user decision (MIT vs Apache-2.0 has long-term implications for any future contributors).
metrics:
  duration: ~22min
  completed: 2026-05-01T23:55:00Z (approx)
  tasks_completed: 3 of 4 (Tasks 1, 3, 4)
  tasks_deferred: 1 (Task 2 -- SignPath application submission, deferred to user with copy-paste package in docs/signpath-application.md)
  bonus_deliverables: 1 (TODO(SignPath OSS) scaffold in release.yml -- per user success criteria, not in plan task list)
  commits: 4
  files_changed: 4 (3 created, 1 modified) + 1 ci scaffold edit = 4 file deltas
---

# Phase 2 Plan 9: SmartScreen walkthrough + SignPath OSS application package

**One-liner:** Shipped INST-05 via a browser-specific SmartScreen walkthrough doc (Edge / Chrome / Firefox + Defender quarantine recovery), a copy-paste-ready SignPath Foundation OSS application tracker (deferred to user submission since the web form requires a real human + GitHub MFA), a guildie-facing README rewrite documenting the tray menu + "tray turned red" recovery, and a `# TODO(SignPath OSS)` scaffold in `release.yml` at the exact future-insertion point between makensis and SHA-256.

## Why

ROADMAP success criterion 5 requires "either signed binary OR documented walkthrough <30s" — and 02-CONTEXT.md locked the unsigned-with-walkthrough path as the Phase 2 default. Plan 02-08 already shipped the unsigned release pipeline (signs: block deliberately omitted from `.goreleaser.yaml`, no signing in `release.yml`); this plan delivered the walkthrough side of the OR-clause and filed the SignPath application package that may eventually flip us to the signed-binary side as a 1.x patch.

The user explicitly added a fourth deliverable beyond the plan's 4 tasks: the `# TODO(SignPath OSS)` scaffold in `release.yml` so the architectural decision (sign in workflow, not goreleaser) survives across multi-phase work. That's tracked here as a bonus deliverable on top of the plan's tasks.

## What changed

### Files created

| File                                                                                          | Purpose                                                                                                                                                                                                                                                                                                                                                                                                |
| --------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `docs/smartscreen-walkthrough.md`                                                             | 77-line browser-specific install walkthrough. Path A (Edge: Keep → More info → Run anyway, ~25s). Path B (Chrome: "Keep / Discard" prompt → identical SmartScreen wall). Path C (Firefox: no MOTW = no SmartScreen by default; Defender may still flag at runtime). Defender-quarantine recovery section (Win Security → Protection history → Restore + Allow on device). MOTW technical explanation. |
| `docs/signpath-application.md`                                                                | 138-line SignPath Foundation OSS application tracker. Status: DEFERRED — pending user submission. Contains a complete copy-paste application package (project name, description, repo URL, license field, build/release process, code-review process, MFA confirmation steps, estimated release frequency). Approval workflow + denial fallback (Certum OSS €69 + €30/yr; never EV).                |
| `.planning/phases/02-watcher-robustness-schema-lock/02-09-SUMMARY.md`                         | This file.                                                                                                                                                                                                                                                                                                                                                                                            |

### Files modified

| File                            | Change                                                                                                                                                                                                                                                                                                                                                                                  |
| ------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `README.md`                     | Phase-1-skeleton stub (140 lines, "repo skeleton only" disclaimer, install-will-land-in-Plan-08 placeholder) → 106-line guildie-facing v1 onboarding doc. Tray menu reference table (Status / Open Workbook / Open log folder / Check for updates / Change Workbook / Continue setup / Reauthorize / Quit). "Tray turned red — what now?" recovery section covering all 3 failure modes. Cross-links to all 4 docs. Auto-update + uninstall + code-signing rationale. File system layout table. |
| `.github/workflows/release.yml` | Added 36-line `# TODO(SignPath OSS)` comment block between "Build NSIS installer" (line 160) and "Compute SHA-256 sums" (now line 206). Documents required secret, signed=false → signed=true manifest flip, and the explicit "do NOT add signs: to .goreleaser.yaml" reinforcement. No active signing — comment block only.                                                              |

### CI workflow scaffold position (verbatim, abridged)

```yaml
      - name: Build NSIS installer (pre-manifest)
        ...

      # TODO(SignPath OSS): wire SignPath signing here, between makensis
      # and Compute SHA-256 sums.
      ...

      - name: Compute SHA-256 sums (installer + bare binary)
```

This is the exact insertion point Plan 02-08's SUMMARY identified ("a workflow STEP between makensis and SHA-256, NOT inside .goreleaser.yaml").

## Commits

| Task             | Hash      | Message                                                              |
| ---------------- | --------- | -------------------------------------------------------------------- |
| 1                | `4d6c2c4` | docs(02-09): add SmartScreen walkthrough with browser-specific paths |
| 2 + 3 (combined) | `6a277de` | docs(02-09): add SignPath OSS application tracker + copy-paste package |
| 4                | `19e4f3b` | docs(02-09): rewrite README for guildie-facing onboarding            |
| Bonus            | `ba5bc47` | ci(02-09): add SignPath OSS scaffold between makensis and SHA-256    |

## Verification results

### Task 1 (`docs/smartscreen-walkthrough.md`)

| Acceptance check                                  | Required | Actual | Result |
| ------------------------------------------------- | -------- | ------ | ------ |
| File exists                                       | yes      | yes    | PASS   |
| `## Path A` + `## Path B` + `## Path C`           | =3       | 3      | PASS   |
| `Run anyway`                                      | >=2      | 6      | PASS   |
| `Keep`                                            | >=2      | 3      | PASS   |
| `Mark-of-the-Web` or `MOTW`                       | >=1      | 2      | PASS   |
| `<30 seconds` or `under 30 seconds`               | >=1      | 2      | PASS   |
| `signpath-application.md` cross-link              | >=1      | 3      | PASS   |
| Line count                                        | >=60     | 77     | PASS   |
| Browser names (Edge / Chrome / Firefox)           | =3       | 3      | PASS   |

### Task 3 (`docs/signpath-application.md`) — combined with deferred Task 2

| Acceptance check                                  | Required | Actual | Result |
| ------------------------------------------------- | -------- | ------ | ------ |
| File exists                                       | yes      | yes    | PASS   |
| `**Status:**`                                     | =1       | 1      | PASS   |
| `smartscreen-walkthrough.md` cross-link           | >=2      | 2      | PASS   |
| `.goreleaser.yaml` mention                        | >=1      | 3      | PASS   |
| `Certum OSS` mention                              | >=1      | 2      | PASS   |
| `disable: true` mention                           | >=1      | 2      | PASS   |
| Line count                                        | >=30     | 138    | PASS   |

### Task 4 (`README.md`)

| Acceptance check                                  | Required | Actual | Result |
| ------------------------------------------------- | -------- | ------ | ------ |
| `smartscreen-walkthrough.md` link                 | >=1      | 1      | PASS   |
| `build-and-install.md` link                       | >=1      | 2      | PASS   |
| `oauth-setup.md` link                             | >=1      | 2      | PASS   |
| `signpath-application.md` link                    | >=1      | 1      | PASS   |
| `tray turned red` (case-insensitive)              | >=1      | 2      | PASS   |
| `Reauthorize`                                     | >=2      | 2      | PASS   |
| `Check for updates`                               | >=2      | 2      | PASS   |
| `releases/latest` download link                   | >=1      | 1      | PASS   |
| Line count                                        | >=50     | 106    | PASS   |

### Bonus deliverable (`.github/workflows/release.yml` SignPath scaffold)

| Check                                                     | Required | Actual | Result |
| --------------------------------------------------------- | -------- | ------ | ------ |
| `TODO(SignPath OSS)` comment block present                | =1       | 1      | PASS   |
| Position: between `Build NSIS installer` and `Compute SHA-256` | yes  | line 170 (between 160 and 206) | PASS |
| No `signs:` block in `release.yml`                        | =0       | 0      | PASS   |
| No `signs:` block in `.goreleaser.yaml`                   | =0       | 0      | PASS   |
| `go build ./...`                                          | exit 0   | exit 0 | PASS   |

## Task 2 deferral — SignPath application

**Status: DEFERRED to user.**

Task 2 in PLAN.md is a `checkpoint:human-action` requiring submission of the SignPath Foundation web form at <https://signpath.org/foundation>. Per the user's executor invocation (`<plan_specific_notes>`), the executor is NOT to block on this checkpoint but is instead to ship a complete copy-paste-ready application package the user can submit manually. That package now lives at `docs/signpath-application.md`.

### What the user needs to do

1. **Add a LICENSE file at repo root.** Currently MISSING. SignPath requires an OSI-approved license. Recommend MIT (short, permissive, conventional Go) — copy from <https://choosealicense.com/licenses/mit/> with `2026 Joe Bowen` as the copyright line. Commit + push.
2. **Visit** <https://signpath.org/foundation>.
3. **Verify GitHub MFA is on** at <https://github.com/settings/security>.
4. **Open `docs/signpath-application.md`** and copy each verbatim block from the "Copy-paste application package" section into the corresponding form field.
5. **Submit.** Save the confirmation email.
6. **Update `docs/signpath-application.md`** with the submission date + reference number + contact thread. Commit + push as `docs(02-09): record SignPath submission YYYY-MM-DD`.

### Application checklist (for user reference)

- [ ] LICENSE file added at repo root (suggested: MIT)
- [ ] GitHub MFA confirmed on at github.com/settings/security
- [ ] Visited <https://signpath.org/foundation>
- [ ] Submitted the form using docs/signpath-application.md verbatim values
- [ ] Saved confirmation email
- [ ] Updated docs/signpath-application.md with submission date + reference number

If denied: per docs/signpath-application.md "If denied" section, fall back to Certum OSS (€69 + €30/yr) or stay unsigned indefinitely. Never buy EV.

## Deviations from Plan

### Process deviations

**1. [Rule 3] Combined Task 2 + Task 3 into a single deliverable + a single commit (`6a277de`)**
- **Found during:** Task 2 (the human-action checkpoint)
- **Issue:** Task 2 is a `checkpoint:human-action` that the executor cannot complete (real human + MFA required). Task 3 produces the tracker doc. Per the user's executor invocation, the right pattern is to write the tracker doc with a Status of DEFERRED + a complete copy-paste application package, and document the submission as the user's manual step.
- **Fix:** Wrote `docs/signpath-application.md` once, with the Status field set to `DEFERRED — pending user submission`, and an embedded "Copy-paste application package" section containing every verbatim form field. The user can submit + then push a follow-up commit to update Status from DEFERRED to FILED.
- **Files modified:** `docs/signpath-application.md` (created)
- **Commit:** `6a277de`
- **Risk:** Low. Phase 2 does NOT block on signing per CONTEXT.md; the walkthrough is the documented INST-05 path.

### Critical functionality added (Rule 2)

**2. [Rule 2 - Bonus deliverable per user success criteria] TODO(SignPath OSS) scaffold in release.yml**
- **Found during:** post-Task-4 review of user's success criteria checklist
- **Issue:** The user's success criteria explicitly required: "`.github/workflows/release.yml` has a `# TODO(SignPath OSS)` scaffolded comment block at the right position (between makensis and SHA-256, NOT inside `.goreleaser.yaml`)". This is not in the plan's 4 tasks but is a hard success-criteria requirement.
- **Fix:** Added a 36-line `# TODO(SignPath OSS)` comment block at line 170 of `release.yml`, between "Build NSIS installer" (line 160) and "Compute SHA-256 sums" (line 206). The block documents the required secret, the signed=false → signed=true manifest flip, and the architectural decision to NOT re-introduce signs: in goreleaser config. No active signing — comment only.
- **Files modified:** `.github/workflows/release.yml`
- **Commit:** `ba5bc47`
- **Risk:** Zero. Comment block, no execution change.

### Auto-fixed Issues

None. The plan was implemented as written for Tasks 1, 3, 4.

### Wording divergence (informational, not a deviation)

PLAN.md Task 1's authored content uses "Windows protected your PC" as the SmartScreen panel title. The Phase 1 build-and-install.md uses "Microsoft Defender SmartScreen prevented an unrecognized app from starting". Both are real Windows wordings (the title shifted around Win11 22H2). The new walkthrough doc presents BOTH wordings in Path A step 3 ("blue panel titled **'Microsoft Defender SmartScreen prevented an unrecognized app from starting'** (older Win11 builds say **'Windows protected your PC'** — same dialog, same buttons)") so the doc is correct regardless of which Win11 build the guildie is on.

### Authentication gates

None.

## Paranoia checks

- `go build ./...` exits 0 (no Go code touched, but verified clean per success criteria)
- `git diff --diff-filter=D --name-only HEAD~4 HEAD` returns empty (no accidental file deletions across the 4 commits)
- `git status --short` shows only pre-existing PROJECT.md modification + `.claude/` untracked dir (both unrelated to Plan 02-09; carried in from before this plan started)
- `grep -nE '^\s*signs:' .github/workflows/release.yml` returns no match (no signs: block anywhere in workflow)
- `grep -nE '^\s*signs:' .goreleaser.yaml` returns no match (no signs: block in goreleaser config — Plan 02-08 omission preserved)
- No EV-cert recommendation anywhere in the new docs (locked decision per CONTEXT.md preserved)
- TODO(SignPath OSS) scaffold sits at the exact insertion point Plan 02-08 SUMMARY documented (between makensis and SHA-256)

## What's unblocked downstream

- **Wave 8 (Plan 02-10 soak)** — sole Wave 7 plan complete; Wave 8 is unblocked.
- **Phase 2 success criterion 5 (INST-05)** — MET via documented walkthrough <30s. The OR-clause's "documented walkthrough" branch is satisfied; the "signed binary" branch remains an async track via SignPath OSS.
- **Future SignPath enablement** — single workflow-step insertion at the documented `TODO(SignPath OSS)` block in `release.yml`. No Go code changes; no goreleaser changes; no architectural rework.
- **Phase 5 DOC-03** — Phase 5 owns onboarding screenshots/video; the walkthrough's `*Screenshot placeholder: ...*` lines are the markers Phase 5 will replace with real PNGs.

## Known stubs

The walkthrough doc contains three explicit `*Screenshot placeholder: ...*` lines:
- `edge-smartscreen-1.png` (the blue panel with "More info" highlighted)
- `edge-smartscreen-2.png` (after clicking More info — "Run anyway" highlighted)
- `chrome-keep-discard.png` (the bottom-bar prompt)

These are intentional per PLAN.md Task 1's `<action>` note: "actual screenshots are deferred to Phase 5 polish (per ROADMAP — Phase 5 owns onboarding screenshots/video for DOC-03). For Phase 2, placeholder text is acceptable. The walkthrough's TEXT is the Phase 2 deliverable." Phase 5 DOC-03 will resolve these.

## Threat Flags

None. This plan is documentation + a CI comment block; no new network endpoints, auth paths, file access patterns, or schema changes.

## Self-Check: PASSED

Files exist:
- FOUND: `docs/smartscreen-walkthrough.md` (4d6c2c4)
- FOUND: `docs/signpath-application.md` (6a277de)
- FOUND: `README.md` (19e4f3b, modified)
- FOUND: `.github/workflows/release.yml` (ba5bc47, scaffold added)

Commits exist:
- FOUND: `4d6c2c4` docs(02-09): add SmartScreen walkthrough with browser-specific paths
- FOUND: `6a277de` docs(02-09): add SignPath OSS application tracker + copy-paste package
- FOUND: `19e4f3b` docs(02-09): rewrite README for guildie-facing onboarding
- FOUND: `ba5bc47` ci(02-09): add SignPath OSS scaffold between makensis and SHA-256
