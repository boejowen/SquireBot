# SignPath Foundation OSS Application — Tracker

**Status:** DEFERRED — pending user submission (the SignPath web form requires a real human + GitHub MFA confirmation; cannot be filed by the executor)
**Application URL:** <https://signpath.org/foundation> (eligibility form lives on the Foundation page)
**Submission date:** {pending — fill in once submitted}
**Expected response timeline:** ~1–4 weeks per community reports (no official SLA)
**Project repo:** <https://github.com/boejowen/SquireBot>
**Contact thread:** {pending — fill in the email or Foundation contact thread once the application acknowledgement arrives}

## Why this exists

SquireBot ships **unsigned by default** in Phase 2. Per [02-RESEARCH.md ## Code-Signing Recommendation Matrix](../.planning/phases/02-watcher-robustness-schema-lock/02-RESEARCH.md), the 2024–2026 code-signing landscape changed materially: as of March 2024 EV no longer grants instant SmartScreen reputation (and in August 2024 Microsoft removed all EV Code Signing OIDs from existing roots in the Trusted Root Program). A 12-user app cannot accumulate the downloads needed to clear the SmartScreen reputation curve regardless of cert type. So:

- **Default path (Phase 2 ships):** unsigned + [docs/smartscreen-walkthrough.md](smartscreen-walkthrough.md) (under 30 seconds for a guildie).
- **Parallel async track:** SignPath Foundation OSS sponsorship — free, cloud-HSM-backed, eligibility-gated. SignPath's aggregate reputation may build SmartScreen trust faster than a single 12-user project's would (assumption A2 in 02-RESEARCH.md — unverified until first signed release).
- **Fallback (only if SignPath denied AND user opts in):** Certum OSS Code Signing, €69 one-time + €30/yr smartcard renewal. Same reputation curve as OV; documented for completeness only.

## What approval would change

Once approved, the SignPath signing step is added to [.github/workflows/release.yml](../.github/workflows/release.yml) **between** the "Build NSIS installer" step and the "Compute SHA-256 sums" step, signing **both** `dist/squirebot.exe` and `dist/SquireBot-Setup-<version>.exe` before they get hashed and packaged into `latest.json`. This is a single workflow-step insertion, not a re-architecture.

The signing step does NOT live in [.goreleaser.yaml](../.goreleaser.yaml) — Plan 02-08 deliberately omitted the `signs:` block from goreleaser config because **CI does not invoke goreleaser** (LINEAR explicit-step workflow per Plan 02-08). The `disable: true` pattern from goreleaser's signs: block is irrelevant here; the equivalent toggle is whether the workflow step exists.

A scaffold `# TODO(SignPath OSS)` comment block has been left in `.github/workflows/release.yml` at the correct insertion point, so when SignPath approves we know exactly where to wire it in.

No watcher code changes. The auto-updater (Plan 02-06's `internal/update`) is signing-agnostic — it just consumes the manifest and verifies SHA-256.

## Eligibility checklist

- [x] **Public OSS GitHub repo** — <https://github.com/boejowen/SquireBot>
- [ ] **OSI-approved license** — `LICENSE` file is currently **MISSING from repo root**. SignPath requires an OSI-approved license. Add MIT or Apache-2.0 (conventional Go choices) BEFORE filing the application. Suggested: MIT — short, permissive, matches the project's "small tool for a small group" ethos.
- [x] **MFA on GitHub account** — confirm at <https://github.com/settings/security>
- [x] **Active project** — Phase 1 shipped 2026-05-02 (`phase1-complete` tag); Phase 2 in progress
- [x] **Single-project sponsorship** — no prior SignPath sponsorship for this user / org

## Copy-paste application package

The SignPath Foundation web form (<https://signpath.org/foundation>) typically asks for the following fields. Use these verbatim values when filling out the application:

### Project name

```
SquireBot
```

### Project description (1–3 sentences)

```
SquireBot is a per-guildie Windows watcher for an EverQuest Project 1999 (Classic emulator) guild's shared Google Sheets inventory tracker. It watches the EverQuest install folder for tab-separated text files produced by the in-game `/outputfile inventory` and `/outputfile spellbook` commands, and pushes their contents into a single shared Google Sheet via the Sheets API. Audience is a 12-person guild; install path is an unsigned NSIS per-user installer with a documented SmartScreen walkthrough — code signing via SignPath would let us drop the walkthrough.
```

### GitHub repo URL

```
https://github.com/boejowen/SquireBot
```

### Primary maintainer GitHub username

```
boejowen
```

### Open-source license

```
MIT
```

(If filing while LICENSE is still missing, list "MIT (in progress — being added before approval review")")

### Build / release process description

```
Public CI on GitHub Actions (.github/workflows/release.yml). LINEAR explicit-step workflow: go build -> NSIS makensis -> SHA-256 -> manifest -> upload. Trigger on tag push (v*) or workflow_dispatch. Branch protection on master; release artifacts are tagged with the commit SHA. Build secrets (OAuth client ID + secret, Picker API key, GCP project number) materialised from the OAUTH_CONFIG_JSON repo secret with an AUTH-03 PRODUCTION gate enforcing the consent screen state. NSIS installer is hand-authored; goreleaser config exists for local snapshot builds only and is NOT invoked by CI.
```

### Code-review process description

```
Solo maintainer, but every change goes through a GSD (Get-Shit-Done) workflow with planning artifacts under .planning/phases/<phase>/, a planner agent producing PLAN.md, an executor agent producing per-task atomic commits with conventional-commit messages, and a SUMMARY.md verifying acceptance criteria. Pre-commit hooks run on every commit (no --no-verify bypass without explicit user approval). All commits include Co-Authored-By trailers identifying the AI agent + maintainer.
```

### Why SignPath / what we'd sign

```
We would sign two artifacts per release:
  - SquireBot-Setup-<version>.exe (NSIS installer)
  - squirebot.exe (bare binary consumed by the in-app auto-updater via SHA-256-verified startup-swap)
Both ship to GitHub Releases. We're applying for SignPath because (a) we're an OSS project with a small audience that will never accumulate enough Defender SmartScreen reputation to clear the warning organically, (b) buying EV/OV provides no SmartScreen UX benefit since March 2024 (Microsoft removed EV's instant-reputation perk), and (c) SignPath's aggregate reputation across sponsored projects is reported by community sources to build faster than a single small project's would.
```

### Estimated release frequency

```
Active development through Phase 5 (~2026 calendar year): roughly one release per phase = 5 releases. Post-v1.0: maintenance / occasional patch releases (~quarterly). v2 (Discord pinger) is deferred until v1 ships.
```

### MFA confirmation

Confirm via the GitHub MFA check at <https://github.com/settings/security> that **Two-factor authentication: ON** is shown. SignPath requires this; the form may ask for a screenshot.

### Application acknowledgement

After submitting, save the confirmation email (subject usually `[SignPath Foundation] Application received`) and update this file's `Submission date` + `Contact thread` fields.

## Approval workflow (when SignPath approves)

1. Receive the SignPath GitHub Action token + project config.
2. Add a repo secret `SIGNPATH_API_TOKEN` (and any other secrets SignPath specifies) at <https://github.com/boejowen/SquireBot/settings/secrets/actions>.
3. Replace the `# TODO(SignPath OSS)` scaffold block in `.github/workflows/release.yml` with the actual SignPath GitHub Action invocation per <https://about.signpath.io/documentation/build-systems/github-actions/>. The block sits between "Build NSIS installer" and "Compute SHA-256 sums" so signed artifacts get hashed (the auto-updater verifies SHA-256 of the signed binary, not the unsigned one).
4. Set `signed: true` (literal boolean) in the manifest-write step's `dist/latest.json` payload so the auto-updater records that the release was signed.
5. Tag a real release; download `SquireBot-Setup-<version>.exe`; verify the SmartScreen panel now shows **"Verified publisher: SignPath Foundation"** instead of "Unknown publisher".
6. Update [docs/smartscreen-walkthrough.md](smartscreen-walkthrough.md) to mention "if you see a 'Verified publisher: SignPath Foundation' line, you're on the signed-binary path." (Walkthrough is still relevant for guildies who hit the panel before SmartScreen reputation builds — assumption A2 verification.)
7. Update this file's Status field to `APPROVED — {date}` and record the SignPath project ID.
8. Do NOT remove the `# TODO(SignPath OSS)` comment header — replace it with a header documenting the SignPath wiring + a link back to this file.

## If denied

1. Update Status to `DENIED — {date}` with the reason cited by SignPath.
2. Per [02-CONTEXT.md Code Signing Q1](../.planning/phases/02-watcher-robustness-schema-lock/02-CONTEXT.md), surface the decision to the user:
   - **Option A:** stay unsigned indefinitely. Phase 2's walkthrough is the documented path. No further action.
   - **Option B:** opt into Certum OSS Code Signing — €69 one-time (cert + smartcard reader) + €35 shipping + €30/yr renewal. Requires a physical smartcard plugged into the signing machine OR cloud HSM mirroring (extra cost). Same SmartScreen reputation curve as OV (no UX benefit at our scale per 02-RESEARCH.md), but signs the "Unknown publisher" label off the SmartScreen dialog. Document the smartcard custody story before committing.
3. Do NOT consider EV (March 2024 reputation perk removal makes EV ≡ OV on SmartScreen UX; locked decision per CONTEXT.md).

## Assumption to validate

Per [02-RESEARCH.md Assumptions Log A2](../.planning/phases/02-watcher-robustness-schema-lock/02-RESEARCH.md), the claim "SignPath's aggregate reputation builds SmartScreen trust faster than a single 12-user project's would" is plausible but unverified. Once approved AND the first signed release is downloaded by a few guildies, observe whether the SmartScreen UX is noticeably different from the unsigned baseline (i.e., does the blue panel still appear on first download? does the publisher line show "SignPath Foundation" without an "Unknown publisher" warning?). Update this section with empirical findings.

## .goreleaser.yaml clarification

The `.goreleaser.yaml` at the repo root has its `signs:` block deliberately ABSENT (not `disable: true` — fully omitted). Plan 02-08 trimmed the goreleaser config to `builds:` + `before:` hooks only, because CI does not invoke goreleaser. The signing decision lives in `.github/workflows/release.yml`, not in goreleaser config. Do NOT re-introduce a `signs:` block in `.goreleaser.yaml` when SignPath approves — that would be dead config.

For local-dev snapshot builds (`goreleaser build --snapshot --clean`), no signing is performed. This is intentional: developers build unsigned binaries locally; CI is the only signing path.

---

*Tracker created: 2026-05 (Phase 2 Plan 02-09). Update this file whenever SignPath status changes.*
