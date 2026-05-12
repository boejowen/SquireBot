---
phase: 08-test-infra-persistence-docs
plan: 04
type: execute
wave: 1
depends_on: []
files_modified:
  - .planning/phases/03-apps-script-enrichment-foundation/03-01-SUMMARY.md
  - .planning/phases/03-apps-script-enrichment-foundation/03-02-SUMMARY.md
  - .planning/phases/03-apps-script-enrichment-foundation/03-03-SUMMARY.md
  - .planning/phases/03-apps-script-enrichment-foundation/03-04-SUMMARY.md
  - .planning/phases/04-differentiator-features/04-01-SUMMARY.md
  - .planning/phases/04-differentiator-features/04-02-SUMMARY.md
  - .planning/phases/04-differentiator-features/04-03-SUMMARY.md
  - .planning/phases/04-differentiator-features/04-04-SUMMARY.md
autonomous: true
requirements: [DOC-04]
tags: [docs, summary-backfill, phase-3, phase-4, doc-04, retroactive]
must_haves:
  truths:
    - "8 retroactive SUMMARY.md files exist: 4 in `.planning/phases/03-apps-script-enrichment-foundation/` (03-01..03-04) and 4 in `.planning/phases/04-differentiator-features/` (04-01..04-04)."
    - "Each SUMMARY.md frontmatter follows the Phase 5 template byte-for-byte: `phase`, `plan`, `subsystem`, `tags`, `requires`, `provides`, `affects`, `tech-stack`, `key-files`, `decisions`, `metrics` — in that order."
    - "Each SUMMARY.md `metrics.commits` field lists at least one short-hash that resolves under `git cat-file -e <sha>` (i.e., real commit SHAs sourced from `git log`, not invented)."
    - "Each SUMMARY.md body has the Phase 5 sections: `# Phase N Plan NN: <title>`, `**One-liner:**`, `## What shipped`, `## Deviations from Plan`, `## Schema impact`, `## Verification log`, `## Self-Check: PASSED`, `## Next plan`."
    - "Each SUMMARY.md sources its content from the corresponding `*-PLAN.md` in the same phase directory plus `.planning/milestones/v1.0-ROADMAP.md` (chronological execution log) plus `git log --oneline --grep='03-0N'` / `04-0N` for commit SHAs."
    - "Voice is past-tense, declarative; mirrors Phase 5's voice (per CONTEXT.md Claude's Discretion)."
    - "v1.0 milestone audit's `Phase 3/4 documentation debt` line item is retired by this plan's completion."
  artifacts:
    - path: ".planning/phases/03-apps-script-enrichment-foundation/03-01-SUMMARY.md"
      provides: "Retroactive summary of Phase 3 Plan 01 per the Phase 5 template"
      min_lines: 50
      contains: "phase: 03-apps-script-enrichment-foundation"
    - path: ".planning/phases/03-apps-script-enrichment-foundation/03-02-SUMMARY.md"
      provides: "Retroactive summary of Phase 3 Plan 02"
      min_lines: 50
      contains: "phase: 03-apps-script-enrichment-foundation"
    - path: ".planning/phases/03-apps-script-enrichment-foundation/03-03-SUMMARY.md"
      provides: "Retroactive summary of Phase 3 Plan 03"
      min_lines: 50
      contains: "phase: 03-apps-script-enrichment-foundation"
    - path: ".planning/phases/03-apps-script-enrichment-foundation/03-04-SUMMARY.md"
      provides: "Retroactive summary of Phase 3 Plan 04"
      min_lines: 50
      contains: "phase: 03-apps-script-enrichment-foundation"
    - path: ".planning/phases/04-differentiator-features/04-01-SUMMARY.md"
      provides: "Retroactive summary of Phase 4 Plan 01"
      min_lines: 50
      contains: "phase: 04-differentiator-features"
    - path: ".planning/phases/04-differentiator-features/04-02-SUMMARY.md"
      provides: "Retroactive summary of Phase 4 Plan 02"
      min_lines: 50
      contains: "phase: 04-differentiator-features"
    - path: ".planning/phases/04-differentiator-features/04-03-SUMMARY.md"
      provides: "Retroactive summary of Phase 4 Plan 03"
      min_lines: 50
      contains: "phase: 04-differentiator-features"
    - path: ".planning/phases/04-differentiator-features/04-04-SUMMARY.md"
      provides: "Retroactive summary of Phase 4 Plan 04"
      min_lines: 50
      contains: "phase: 04-differentiator-features"
  key_links:
    - from: ".planning/phases/03-apps-script-enrichment-foundation/03-0N-SUMMARY.md (each)"
      to: ".planning/phases/03-apps-script-enrichment-foundation/03-0N-PLAN.md"
      via: "frontmatter requires/provides/key-files/decisions derived from the corresponding PLAN.md"
      pattern: "^phase: 03-apps-script-enrichment-foundation"
    - from: ".planning/phases/04-differentiator-features/04-0N-SUMMARY.md (each)"
      to: ".planning/phases/04-differentiator-features/04-0N-PLAN.md"
      via: "frontmatter requires/provides/key-files/decisions derived from the corresponding PLAN.md"
      pattern: "^phase: 04-differentiator-features"
    - from: "Each SUMMARY.md `metrics.commits` field"
      to: "git log (master branch)"
      via: "real short-hashes resolvable by git cat-file -e"
      pattern: "commits:\\s*\\d+"
---

<objective>
Backfill the 8 retroactive `SUMMARY.md` files for Phase 3 (4 plans) and Phase 4 (4 plans). Each file follows the Phase 5 SUMMARY.md template byte-for-byte (per D-07) and sources its content from the corresponding `*-PLAN.md` plus `git log` plus `.planning/milestones/v1.0-ROADMAP.md`. After this plan ships, the v1.0 milestone audit's "Phase 3/4 documentation debt" line item is structurally retired.

Purpose: REQUIREMENTS.md DOC-04. The 8 plans shipped during v1.0 (2026-04-30 → 2026-05-04 window) without their SUMMARY artifacts. Phase 5 (and every phase after) wrote SUMMARYs as part of execution; Phase 3 and Phase 4 are the only retroactive holdouts.

Output: 8 new markdown files (each ~80-150 lines), zero code changes, zero test impact. Pure docs surface.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/STATE.md
@.planning/REQUIREMENTS.md
@.planning/phases/08-test-infra-persistence-docs/08-CONTEXT.md
@.planning/phases/08-test-infra-persistence-docs/08-RESEARCH.md
@.planning/phases/08-test-infra-persistence-docs/08-PATTERNS.md

<!-- Canonical template -- clone its frontmatter shape byte-for-byte. -->
@.planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md

<!-- Source-of-truth feeders for each backfill. -->
@.planning/phases/03-apps-script-enrichment-foundation/03-01-PLAN.md
@.planning/phases/03-apps-script-enrichment-foundation/03-02-PLAN.md
@.planning/phases/03-apps-script-enrichment-foundation/03-03-PLAN.md
@.planning/phases/03-apps-script-enrichment-foundation/03-04-PLAN.md
@.planning/phases/04-differentiator-features/04-01-PLAN.md
@.planning/phases/04-differentiator-features/04-02-PLAN.md
@.planning/phases/04-differentiator-features/04-03-PLAN.md
@.planning/phases/04-differentiator-features/04-04-PLAN.md
@.planning/milestones/v1.0-ROADMAP.md

<interfaces>
<!-- Phase 5 SUMMARY.md template -- frontmatter field NAMES + ORDER must match byte-for-byte per D-07. -->

```yaml
---
phase: <phase-dir-name, e.g., 03-apps-script-enrichment-foundation>
plan: <NN, zero-padded>
subsystem: <kebab-case-subsystem-label>
tags: [<4-6 kebab-case tags, first ~2 categorical>]
requires:
  - <upstream-plan-id> (<short rationale>)
provides:
  - "<REQ-ID or capability>: <one-line summary>"
affects:
  - "<downstream-plan or future-phase impact>"
tech-stack:
  added: []                  # NPM packages newly added in this plan; usually [] for backfills
  patterns:
    - "<pattern name>: <explanation>. Locked by RESEARCH §<section>."
key-files:
  created:
    - <path> (<NN lines>)
  modified:
    - <path> (<+NN/-NN lines>; <what-changed-summary>)
decisions:
  - "<terse declarative statement of one decision>"
metrics:
  duration: <~Nmin or ~Nh>     # if unknown, write `unknown (retroactive backfill)`
  completed: <ISO-8601 timestamp>  # if unknown, use the commit-author date from git log
  tasks_completed: <X of Y>
  commits: <N> (<short-hash short-msg>; <short-hash short-msg>; ...)
  files_changed: <N> (<X created + Y modified>, ~<N>00 lines added)
  tests_added: <N>
  trigger_count_after: <N>
  schema_version_after: <N>
  watcher_rebuild_required: <true|false>
---
```

Source ordering: SUMMARY frontmatter follows the PLAN.md's already-recorded `files_created` + `files_modified` arrays; commit shas come from `git log --oneline --since=2026-04-30 --until=2026-05-04 -- apps-script/` filtered by phase number; `decisions` mirrors any deviation notes from STATE.md's Phase 3/4 "Last Session Summary" entries (currently archived in `.planning/milestones/v1.0-ROADMAP.md`).

Variance allowed: `tech-stack.added: []` for nearly all backfills (only 03-01 likely has entries because that's where clasp + esbuild + @types/google-apps-script first landed); `## Threat-register coverage` section is omitted if the source PLAN.md has no `<threat_model>` block (Phase 3 PLANs predate the threat-model convention, so most backfills will omit this section).
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Backfill 4 Phase 3 SUMMARY.md files (03-01 through 03-04)</name>
  <files>.planning/phases/03-apps-script-enrichment-foundation/03-01-SUMMARY.md, .planning/phases/03-apps-script-enrichment-foundation/03-02-SUMMARY.md, .planning/phases/03-apps-script-enrichment-foundation/03-03-SUMMARY.md, .planning/phases/03-apps-script-enrichment-foundation/03-04-SUMMARY.md</files>
  <read_first>
    - .planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md (canonical template -- byte-clone the frontmatter shape)
    - .planning/phases/03-apps-script-enrichment-foundation/03-01-PLAN.md (Phase 3 Plan 01 source-of-truth for `files_created`/`files_modified`/objectives)
    - .planning/phases/03-apps-script-enrichment-foundation/03-02-PLAN.md
    - .planning/phases/03-apps-script-enrichment-foundation/03-03-PLAN.md
    - .planning/phases/03-apps-script-enrichment-foundation/03-04-PLAN.md
    - .planning/milestones/v1.0-ROADMAP.md (chronological execution log -- find the Phase 3 entries for completion timestamps + decision notes)
  </read_first>
  <action>
1. From the repo root, mine commit SHAs for each of the 4 Phase 3 plans:

```bash
git log --oneline --all --grep="03-01" -- apps-script/ | head -10
git log --oneline --all --grep="03-02" -- apps-script/ | head -10
git log --oneline --all --grep="03-03" -- apps-script/ | head -10
git log --oneline --all --grep="03-04" -- apps-script/ | head -10
```

If grep returns nothing for a given plan, widen the search:

```bash
git log --oneline --all --since=2026-04-30 --until=2026-05-04 -- .planning/phases/03-apps-script-enrichment-foundation/
git log --oneline --all --since=2026-04-30 --until=2026-05-04 -- apps-script/
```

Pull commit hashes + author dates per plan; cross-reference with the `files_created` / `files_modified` arrays at the top of each `03-0N-PLAN.md` to disambiguate "which commit touched which plan."

2. For each Phase 3 plan, author the SUMMARY.md by cloning the Phase 5 frontmatter shape verbatim. Use this scaffold (replace `<...>` placeholders):

```yaml
---
phase: 03-apps-script-enrichment-foundation
plan: 0N
subsystem: <kebab-case-derived-from-plan-objective>
tags: [apps-script, <topical-tag-1>, <topical-tag-2>, <req-id>, <req-id>]
requires:
  - <upstream-plan-id-from-PLAN.depends_on> (<short rationale>)
provides:
  - "<REQ-ID>: <one-line summary from PLAN.md objective>"
  - "<artifact>: <what-it-does>"
affects:
  - "<downstream-plan-or-future-phase>: <how-it-extends>"
tech-stack:
  added: <[] for most; populate for 03-01 if it added clasp+esbuild+@types/google-apps-script>
  patterns:
    - "<pattern-name>: <explanation>. Locked by RESEARCH §<section> / PATTERNS §<section>."
key-files:
  created:
    - <path> (<NN lines>)
  modified:
    - <path> (<+NN/-NN lines>; <what-changed>)
decisions:
  - "<terse declarative decision statement>"
metrics:
  duration: unknown (retroactive backfill 2026-05-12)
  completed: <ISO-8601 from git author-date of the plan's terminal commit>
  tasks_completed: <X of Y per PLAN.md task count>
  commits: <N> (<short-hash short-msg>; ...)
  files_changed: <N>
  tests_added: <N>
  trigger_count_after: <look up from STATE.md or ROADMAP.md; if unknown, `<unknown>`>
  schema_version_after: <2 or 3, depending on plan; see PLAN.md or v1.0-ROADMAP.md>
  watcher_rebuild_required: <true|false>
---

# Phase 3 Plan 0N: <Title from PLAN.md>

**One-liner:** <single dense sentence summarizing the plan outcome>.

## What shipped

### Task 1 -- <task-name> (commit `<short-hash>`)

<2-3 paragraphs past-tense; cite line numbers / function names where load-bearing>

### Task 2 -- ...

## Deviations from Plan

None. Plan executed as written.  (OR an itemized list pulled from STATE.md if deviations were recorded.)

## Schema impact

<Path A confirmed. or Schema bumped to N. -- read from PLAN.md `<must_haves>` schema_version field if present.>

## Verification log

```
$ npm test
Tests       <baseline at time of plan close> passed

$ <relevant-greps from PLAN.md verification block>
<expected output>
```

(If verification log is not reconstructible -- "unknown (retroactive); see PLAN.md verification block".)

## Self-Check: PASSED

**Files exist (all N changed):**
- FOUND: `<path>` (<sentinel>)
- ...

**Commits exist:**
- FOUND: `<short-hash>` -- <commit-msg>

## Next plan

`/gsd-execute-phase 3` spawned plan `03-(0N+1)` for <brief next-plan-objective>. (For the terminal plan 03-04: `/gsd-execute-phase 4` opened Phase 4 (Differentiator Features) starting with 04-01.)

---

*Retroactively authored 2026-05-12 by Phase 8 Plan 08-04 (DOC-04). Source artifacts: 03-0N-PLAN.md + git log + .planning/milestones/v1.0-ROADMAP.md.*
```

3. Author 03-01-SUMMARY.md, 03-02-SUMMARY.md, 03-03-SUMMARY.md, 03-04-SUMMARY.md following this scaffold. The amount of detail in `## What shipped` should match what is reconstructible from the PLAN.md + git log -- typically 1-2 paragraphs per task, not full Phase-5-quality narrative. Per CONTEXT D-07 the verification depth is "existence grep gate + spot-check of key-files.created and decisions" -- the backfill bar is "structurally complete and accurate," NOT "feature-quality essay."

4. Append a noticeable retroactive marker to each file (the italicized footer above) so future readers know these are backfills rather than at-time-of-shipping summaries.

5. Verify the 4 files exist and have the required frontmatter keys:

```bash
for n in 01 02 03 04; do
  f=".planning/phases/03-apps-script-enrichment-foundation/03-$n-SUMMARY.md"
  test -f "$f" || { echo "MISSING: $f"; exit 1; }
  for key in phase plan subsystem tags requires provides affects tech-stack key-files decisions metrics; do
    grep -q "^${key}:" "$f" || { echo "MISSING KEY $key in $f"; exit 1; }
  done
done
```

6. Verify commit SHAs in each file resolve:

```bash
for f in .planning/phases/03-apps-script-enrichment-foundation/03-0*-SUMMARY.md; do
  for sha in $(grep -oE "\b[a-f0-9]{7}\b" "$f" | sort -u); do
    git cat-file -e "$sha" 2>/dev/null || echo "UNRESOLVED SHA in $f: $sha"
  done
done
```

Any unresolved SHAs surface as `UNRESOLVED SHA in <file>: <sha>` warnings — these indicate either typos or invented SHAs and must be corrected before commit.

7. Commit:
```bash
git add .planning/phases/03-apps-script-enrichment-foundation/03-0*-SUMMARY.md
git commit -m "docs(08-04): backfill Phase 3 plan summaries (DOC-04)"
```
  </action>
  <verify>
    <automated>
# 4 Phase 3 SUMMARY files exist
for n in 01 02 03 04; do
  test -f ".planning/phases/03-apps-script-enrichment-foundation/03-$n-SUMMARY.md" || exit 1
done

# Each file has all 11 required frontmatter keys
for f in .planning/phases/03-apps-script-enrichment-foundation/03-0[1-4]-SUMMARY.md; do
  for key in phase plan subsystem tags requires provides affects tech-stack key-files decisions metrics; do
    grep -q "^${key}:" "$f" || { echo "FAIL: $f missing $key"; exit 1; }
  done
done

# Phase-slug correct in each
for f in .planning/phases/03-apps-script-enrichment-foundation/03-0[1-4]-SUMMARY.md; do
  grep -q "^phase: 03-apps-script-enrichment-foundation" "$f" || exit 1
done

# Min-line gate (50 lines = frontmatter + skeleton sections at minimum)
for f in .planning/phases/03-apps-script-enrichment-foundation/03-0[1-4]-SUMMARY.md; do
  n=$(wc -l < "$f"); [ "$n" -ge 50 ] || { echo "FAIL: $f has $n lines (< 50)"; exit 1; }
done

# Every committed SHA in any SUMMARY resolves (warn-only -- run loop will exit 0 if all good)
for f in .planning/phases/03-apps-script-enrichment-foundation/03-0[1-4]-SUMMARY.md; do
  for sha in $(grep -oE "\b[a-f0-9]{7}\b" "$f" | sort -u); do
    git cat-file -e "$sha" 2>/dev/null || { echo "FAIL: unresolved sha $sha in $f"; exit 1; }
  done
done
    </automated>
  </verify>
  <acceptance_criteria>
    - 4 files exist: `03-01-SUMMARY.md`, `03-02-SUMMARY.md`, `03-03-SUMMARY.md`, `03-04-SUMMARY.md` under `.planning/phases/03-apps-script-enrichment-foundation/`
    - Each file has 11 frontmatter keys present (`phase`, `plan`, `subsystem`, `tags`, `requires`, `provides`, `affects`, `tech-stack`, `key-files`, `decisions`, `metrics`)
    - Each file's `phase:` line equals `03-apps-script-enrichment-foundation`
    - Each file is at least 50 lines (frontmatter + skeleton body)
    - Every short-hash SHA in each file resolves via `git cat-file -e`
    - `grep -c "Retroactively authored 2026-05-12" .planning/phases/03-apps-script-enrichment-foundation/03-0[1-4]-SUMMARY.md` returns 4 (one per file)
  </acceptance_criteria>
  <done>4 retroactive Phase 3 SUMMARY.md files committed, each with correct frontmatter and real commit SHAs.</done>
</task>

<task type="auto">
  <name>Task 2: Backfill 4 Phase 4 SUMMARY.md files (04-01 through 04-04)</name>
  <files>.planning/phases/04-differentiator-features/04-01-SUMMARY.md, .planning/phases/04-differentiator-features/04-02-SUMMARY.md, .planning/phases/04-differentiator-features/04-03-SUMMARY.md, .planning/phases/04-differentiator-features/04-04-SUMMARY.md</files>
  <read_first>
    - .planning/phases/05-search-onboarding-privacy-polish/05-01-SUMMARY.md (template -- same as Task 1)
    - .planning/phases/04-differentiator-features/04-01-PLAN.md
    - .planning/phases/04-differentiator-features/04-02-PLAN.md
    - .planning/phases/04-differentiator-features/04-03-PLAN.md
    - .planning/phases/04-differentiator-features/04-04-PLAN.md
    - .planning/milestones/v1.0-ROADMAP.md (chronological log -- Phase 4 entries)
  </read_first>
  <action>
1. Repeat the SHA-mining workflow from Task 1, replacing `03-0N` with `04-0N`:

```bash
git log --oneline --all --grep="04-01" -- apps-script/ | head -10
git log --oneline --all --grep="04-02" -- apps-script/ | head -10
git log --oneline --all --grep="04-03" -- apps-script/ | head -10
git log --oneline --all --grep="04-04" -- apps-script/ | head -10
```

2. Author the 4 Phase 4 SUMMARY.md files using the SAME scaffold as Task 1, with `phase: 04-differentiator-features` in the frontmatter. Per CONTEXT D-07 some Phase 4 plans MAY have had `<threat_model>` blocks (Phase 4 introduced eviction sidebar + bank-coin protection, both security-relevant) -- if a source PLAN.md has a `<threat_model>` block, INCLUDE a `## Threat-register coverage` section in the SUMMARY body listing the STRIDE items by ID.

3. Author 04-01-SUMMARY.md, 04-02-SUMMARY.md, 04-03-SUMMARY.md, 04-04-SUMMARY.md. The terminal plan (04-04) is `installTriggers + protectBankCoinCells + monitorCellCount` per 05-01-SUMMARY's `requires:` block -- use that as a cross-check.

4. Verify file existence + frontmatter + SHAs (mirror Task 1 verification commands with the Phase 4 paths):

```bash
for n in 01 02 03 04; do
  f=".planning/phases/04-differentiator-features/04-$n-SUMMARY.md"
  test -f "$f" || { echo "MISSING: $f"; exit 1; }
  for key in phase plan subsystem tags requires provides affects tech-stack key-files decisions metrics; do
    grep -q "^${key}:" "$f" || { echo "MISSING KEY $key in $f"; exit 1; }
  done
done

for f in .planning/phases/04-differentiator-features/04-0*-SUMMARY.md; do
  for sha in $(grep -oE "\b[a-f0-9]{7}\b" "$f" | sort -u); do
    git cat-file -e "$sha" 2>/dev/null || echo "UNRESOLVED SHA in $f: $sha"
  done
done
```

5. Commit:
```bash
git add .planning/phases/04-differentiator-features/04-0*-SUMMARY.md
git commit -m "docs(08-04): backfill Phase 4 plan summaries (DOC-04)"
```
  </action>
  <verify>
    <automated>
for n in 01 02 03 04; do
  test -f ".planning/phases/04-differentiator-features/04-$n-SUMMARY.md" || exit 1
done

for f in .planning/phases/04-differentiator-features/04-0[1-4]-SUMMARY.md; do
  for key in phase plan subsystem tags requires provides affects tech-stack key-files decisions metrics; do
    grep -q "^${key}:" "$f" || { echo "FAIL: $f missing $key"; exit 1; }
  done
done

for f in .planning/phases/04-differentiator-features/04-0[1-4]-SUMMARY.md; do
  grep -q "^phase: 04-differentiator-features" "$f" || exit 1
done

for f in .planning/phases/04-differentiator-features/04-0[1-4]-SUMMARY.md; do
  n=$(wc -l < "$f"); [ "$n" -ge 50 ] || { echo "FAIL: $f has $n lines (< 50)"; exit 1; }
done

for f in .planning/phases/04-differentiator-features/04-0[1-4]-SUMMARY.md; do
  for sha in $(grep -oE "\b[a-f0-9]{7}\b" "$f" | sort -u); do
    git cat-file -e "$sha" 2>/dev/null || { echo "FAIL: unresolved sha $sha in $f"; exit 1; }
  done
done

# v1.0 documentation debt is structurally retired -- all 8 backfills land
total=$(ls .planning/phases/{03-apps-script-enrichment-foundation,04-differentiator-features}/0[3,4]-0[1-4]-SUMMARY.md 2>/dev/null | wc -l)
[ "$total" -eq 8 ] || { echo "FAIL: expected 8 backfill files, found $total"; exit 1; }
    </automated>
  </verify>
  <acceptance_criteria>
    - 4 files exist: `04-01-SUMMARY.md`, `04-02-SUMMARY.md`, `04-03-SUMMARY.md`, `04-04-SUMMARY.md` under `.planning/phases/04-differentiator-features/`
    - Each file has all 11 required frontmatter keys
    - Each file's `phase:` line equals `04-differentiator-features`
    - Each file is at least 50 lines
    - Every short-hash SHA in each file resolves via `git cat-file -e`
    - The combined count of `0[3,4]-0[1-4]-SUMMARY.md` files under the two phase directories is exactly 8 (4 from Task 1 + 4 from Task 2)
  </acceptance_criteria>
  <done>4 retroactive Phase 4 SUMMARY.md files committed; the total Phase-3-and-4 backfill count is 8; v1.0 milestone audit's "Phase 3/4 documentation debt" line item is structurally retired.</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| docs ↔ runtime | None. Pure markdown; not consumed by any executable or build step. |
| docs ↔ git history | SUMMARY commit SHAs must resolve under `git cat-file -e` so future readers can trace claims to commits. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-08-04-01 | Repudiation | Backfill SUMMARY contains invented commit SHAs that don't resolve | mitigate | Verify gate at end of each task: `git cat-file -e <sha>` on every short-hash extracted via `grep -oE '\b[a-f0-9]{7}\b'` from each SUMMARY file. Unresolved SHAs fail the task. |
| T-08-04-02 | Information disclosure | Backfill misrepresents what shipped (claims a feature that wasn't actually delivered) | accept (low risk) | Backfill sources are the PLAN.md (already authoritative) plus `.planning/milestones/v1.0-ROADMAP.md` (already authoritative). The retroactive footer (`*Retroactively authored 2026-05-12...*`) marks each file as best-effort reconstruction, not at-time-of-shipping truth. |
| T-08-04-03 | Tampering | A future contributor edits a backfill SUMMARY to revise history | accept | Standard git audit trail; retroactive footer is the structural signpost. No additional mitigation. |

**Note:** Phase 8 Plan 08-04 is a pure-docs plan; the STRIDE register is intentionally brief per security_enforcement scope (Phase 8 has very low application threat surface for docs).
</threat_model>

<verification>
After both tasks complete:

```bash
# All 8 backfill files exist
for path_pat in "03-apps-script-enrichment-foundation/03" "04-differentiator-features/04"; do
  phase_dir=$(dirname ".planning/phases/$path_pat-XX")
  for n in 01 02 03 04; do
    f=".planning/phases/${path_pat}-$n-SUMMARY.md"
    test -f "$f" || { echo "MISSING: $f"; exit 1; }
  done
done

# Each has the 11 locked frontmatter keys
for f in .planning/phases/03-apps-script-enrichment-foundation/03-0[1-4]-SUMMARY.md .planning/phases/04-differentiator-features/04-0[1-4]-SUMMARY.md; do
  for key in phase plan subsystem tags requires provides affects tech-stack key-files decisions metrics; do
    grep -q "^${key}:" "$f" || { echo "FAIL: $f missing $key"; exit 1; }
  done
done

# Every committed SHA resolves
for f in .planning/phases/03-apps-script-enrichment-foundation/03-0[1-4]-SUMMARY.md .planning/phases/04-differentiator-features/04-0[1-4]-SUMMARY.md; do
  for sha in $(grep -oE "\b[a-f0-9]{7}\b" "$f" | sort -u); do
    git cat-file -e "$sha" || { echo "FAIL: $sha in $f"; exit 1; }
  done
done
```

No code changes; no schema changes; no test impact. Schema-gate verification is trivial (all gates remain at baseline).
</verification>

<success_criteria>
- 8 retroactive SUMMARY.md files exist (4 under Phase 3, 4 under Phase 4).
- Each has the 11 locked frontmatter keys in Phase 5 template order.
- Each is at least 50 lines.
- Each contains at least one verifiable commit SHA.
- Each marks itself as retroactive via the italicized footer.
- v1.0 milestone audit's "Phase 3/4 documentation debt" line item is structurally retired.
- Zero changes to apps-script code; zero schema impact; zero new tests.
</success_criteria>

<output>
After completion, create `.planning/phases/08-test-infra-persistence-docs/08-04-SUMMARY.md` per the Phase 5 template. This is the at-time-of-shipping summary of Plan 08-04 itself (the backfill plan), separate from the 8 retroactive files this plan creates. The 08-04 SUMMARY's `key-files.created` block lists the 8 backfilled files; its `metrics.commits` lists this plan's own commits.
</output>
