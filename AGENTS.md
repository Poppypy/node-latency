# Repository Guidelines

## Project Structure & Module Organization
- `main.go` and `app.go` define the Wails desktop entrypoint and app bindings.
- `internal/` contains backend domain logic (parsing, latency testing, export helpers); keep non-UI Go code here.
- `frontend/` is the Vue 3 + TypeScript UI (views, stores, components, assets).
- `build/` holds Wails build outputs (especially `build/bin/` executables).
- `README.md` and `README.zh-CN.md` are user-facing docs; update both when behavior changes.

## Build, Test, and Development Commands
- `wails dev` — run full desktop app with live frontend/backend reload.
- `wails build` — produce production package and installer artifacts.
- `cd frontend && npm install` — install frontend dependencies.
- `cd frontend && npm run dev` — run frontend-only dev server.
- `cd frontend && npm run build` — build frontend assets consumed by Wails.
- `go test ./...` — run backend Go tests.

## Coding Style & Naming Conventions
- Go: format with `gofmt` (or `go fmt ./...`), package names short/lowercase, exported identifiers `PascalCase`.
- TypeScript/Vue: 2-space indentation, components in `PascalCase.vue`, composables/helpers in `camelCase`.
- Prefer descriptive names such as `latencyResult`, `subscriptionParser`; avoid one-letter names.
- Keep protocol constants/config values centralized; avoid hardcoded URLs/timeouts in UI code.

## Testing Guidelines
- Backend tests should live next to implementation files as `*_test.go`.
- Frontend changes should include at least manual verification in `wails dev`; add automated tests if test setup exists.
- Focus coverage on parsing correctness, timeout/concurrency behavior, and export formatting.
- Before PR: run `go test ./...` and `cd frontend && npm run build`.

## Commit & Pull Request Guidelines
- Current history uses very short messages; prefer clear, imperative commits moving forward.
- Suggested pattern: `feat: add subscription dedupe`, `fix: handle empty proxy name`.
- Keep commits scoped (backend parsing, frontend table, build config, etc.).
- PRs should include: summary, motivation, test steps, and screenshots/GIFs for UI changes.
- Link related issues and note breaking changes explicitly.

## Security & Configuration Tips
- Do not commit private subscription URLs, tokens, or local test data.
- Treat `mihomo-windows-amd64-v3.exe` updates as security-sensitive; verify source and version.
- Prefer environment/config inputs over embedding secrets in code or docs.
