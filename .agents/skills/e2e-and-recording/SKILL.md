---
name: e2e-and-recording
description: Run Playwright E2E tests, record demo walkthrough videos (.webm/.mp4/.gif), and capture bilingual documentation screenshots (.png) for twsnmpneo. Use whenever asked to run E2E tests, verify UI flows, inspect failures via Playwright trace/screenshots, record demo videos, or update docs/images screenshots.
---

# twsnmpneo E2E Testing & Screen Recording

## Prerequisites & Sandbox Note

- Playwright (`e2e/`), `mise`, `pnpm`, and Playwright-managed Chromium (`~/Library/Caches/ms-playwright`) live outside the standard sandbox.
- When invoking `mise`, `pnpm`, or Playwright commands from Antigravity, try sandboxed first if unsure, or use `BypassSandbox: true` (user-approved for this workflow) with a clear `toolSummary`.
- Never depend on external `./data` symlink data; E2E scripts automatically spawn an isolated `twsnmpneo` process in `e2e/.tmp-data-*` on ephemeral high ports and seed demo nodes, switches, lines, pollings, Public Dashboard, UDP Syslog, and OTLP metrics/traces/logs via API.

## Quick Commands (`mise`)

Run from workspace root `/Users/ymi/prj/twsnmp/twsnmpneo`:

1. **First-time setup** (downloads npm deps + Playwright Chromium):
   ```sh
   mise run e2e-install
   ```
2. **Full build + headless E2E suite**:
   ```sh
   mise run e2e
   ```
3. **Fast E2E re-run** (when `./bin/twsnmpneo` is already up to date):
   ```sh
   mise run e2e-only
   ```
4. **Headed E2E debugging**:
   ```sh
   mise run e2e-headed
   ```
5. **Record demo walkthrough video** (`e2e/output/overview-ja.{webm,mp4,gif}`):
   ```sh
   mise run record-demo
   ```
   Optional scenarios / language (`--scenario=overview|public-dashboard`, `--lang=ja|en`):
   ```sh
   cd e2e && pnpm run record:demo -- --scenario=public-dashboard --lang=en
   ```
6. **Capture bilingual documentation screenshots** (`ja` and `en`):
   Preview into `e2e/output/docs-preview/{ja,en}/`:
   ```sh
   mise run capture-docs
   ```
   Write directly into `docs/images/{ja,en}/` (after reviewing preview PNGs):
   ```sh
   cd e2e && DOCS_WRITE=1 pnpm run capture:docs
   ```
   Filter specific targets or locale:
   ```sh
   cd e2e && pnpm run capture:docs -- --locales=ja --targets=map_overview,node_detail
   ```

## Architecture Overview

- `e2e/playwright.config.ts` — Chromium with SwiftShader/ANGLE WebGL enabled (`1440x900`, video/trace on failure).
- `e2e/fixtures/test-server.ts` — Spawns `bin/twsnmpneo` (or `TWSNMPNEO_BIN`) with free ephemeral TCP/UDP ports (`-http-port 0`, explicit high ports for Syslog/Trap/NetFlow/sFlow/OTel/MQTT) and temp datadir; supports attaching to a running server via `E2E_BASE_URL=http://127.0.0.1:8080`.
- `e2e/fixtures/seed-data.ts` — Seeds admin/readonly users, nodes, L2 switch network, lines, drawitems, pollings, Public Dashboard (`e2e-demo-key`), OTLP HTTP/JSON metrics/traces/logs, and UDP Syslog packets.
- `e2e/fixtures/auth.ts` — UI login helper, MapLibre external tile offline mocking (`openstreetmap.org`, `gsi.go.jp`, `cartocdn.com`), and pageerror/console/HTTP5xx error monitor.
- `e2e/tests/*.spec.ts` — E2E specs for Auth/RBAC, 9-view smoke + 3D VPanel + ConfigModal, Map canvas & context menu CRUD, Public Dashboard & Drama Alert, and i18n/theme persistence.

## Troubleshooting Failures

- On test failure, inspect `e2e/test-results/` screenshots/videos or open `e2e/playwright-report/index.html` (`mise run e2e-report` or `cd e2e && pnpm exec playwright show-report`).
- Verify captured PNGs or videos directly in Antigravity via `read_file` on `e2e/output/docs-preview/.../*.png` or `e2e/test-results/.../*.png`.
- Note that CI builds Linux headless binaries with SwiftShader WebGL and runs `pnpm exec playwright install --with-deps chromium`.
