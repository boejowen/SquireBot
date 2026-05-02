# SquireBot — coordination Makefile.
#
# Most build/release work is done by .github/workflows/release.yml +
# docs/build-and-install.md (PowerShell-driven on Windows). This
# Makefile only covers cross-cutting coordination targets that don't
# fit cleanly in those places.
#
# Phase 2 added two coordination targets:
#   soak-7d     -- prints the 7-day soak schedule + runbook reference.
#   soak-assert -- runs the AllPhase2 log-assertion sweep (Day 7).
#
# Both targets are Windows-friendly: soak-7d is pure echo (works in
# any shell), soak-assert shells out to pwsh.

.PHONY: soak-7d soak-assert

soak-7d:
	@echo "Phase 2 soak runbook: docs/soak-runbook.md"
	@echo "This is a 7-day live soak — see runbook for setup + day-by-day procedure."
	@echo ""
	@echo "Schedule:"
	@echo "  Day 0 (T+0):   Setup -- clean Win11 box, install latest tag, autostart verified, two test files dropped."
	@echo "  Day 1 (T+24h): Quota throttle injection -- scripts/soak/inject-quota-throttle.md"
	@echo "  Day 4 (T+96h): invalid_grant injection -- scripts/soak/inject-invalid-grant.md"
	@echo "  Day 6 (T+144h): Corrupt update injection -- scripts/soak/inject-corrupt-update.md"
	@echo "  Day 7 (T+168h): Final sweep -- pwsh ./scripts/soak/grep-log-assertions.ps1 -Scenario AllPhase2"
	@echo ""
	@echo "After all assertions pass, copy the runbook to .planning/phases/02-watcher-robustness-schema-lock/SOAK-REPORT-{date}.md and mark each [ ] as [x] PASS."

soak-assert:
	@echo "Running Phase 2 soak assertions (last 7 days of log)..."
	pwsh -NoProfile -File ./scripts/soak/grep-log-assertions.ps1 -Scenario AllPhase2
