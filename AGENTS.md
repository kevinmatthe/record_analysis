# Repository Guidelines

## Project Structure & Module Organization

This repository currently contains a minimal project scaffold:

- `README.md` documents the project name and should be expanded with user-facing usage notes.
- `AGENTS.md` provides contributor and automation guidance.
- `.agents/` and `.codex/` are present for local agent configuration; do not treat them as application source.

No source, tests, or assets directories exist yet. When adding code, prefer `src/` for modules, `tests/` for automated tests, and `assets/` or `data/` for static inputs. Keep generated outputs out of source directories unless they are intentionally versioned fixtures.

## Build, Test, and Development Commands

There is no project-specific build or test command yet. Before adding language tooling, document commands in `README.md` and keep this file in sync.

Useful repository commands:

- `git status --short` shows pending changes.
- `git log --oneline -8` reviews recent commit style.
- `rg --files` lists tracked and untracked files quickly.

If a build system is introduced, prefer standard commands such as `make test`, `npm test`, or `pytest`, and ensure they run from the repository root.

## Coding Style & Naming Conventions

Follow the conventions of the language or framework introduced in `src/`. Use descriptive filenames and lowercase module names with separators where appropriate, for example `record_parser.py` or `record-parser.ts`. Avoid broad utility modules unless repeated behavior justifies them.

Use formatter and linter defaults when available, and commit configuration files with the first code that depends on them. Keep comments concise and focused on non-obvious logic.

## Testing Guidelines

No test framework is configured yet. Add tests alongside the first meaningful implementation. Prefer a dedicated `tests/` directory and name tests after behavior, for example `test_record_parser.py` or `record-parser.test.ts`.

Tests should cover parsing rules, edge cases, and any data transformation behavior. Include sample fixtures only when they are small, stable, and safe to publish.

## Commit & Pull Request Guidelines

Current history contains only `first commit`, so no detailed convention is established. Use short, imperative commit messages such as `Add record parser` or `Document test workflow`.

Pull requests should include a brief summary, the commands run for verification, and any relevant issue links. Add screenshots only for UI changes. Note any skipped tests or missing tooling explicitly.

## Security & Configuration Tips

Do not commit credentials, private records, or large raw datasets. Prefer environment variables for secrets and document required configuration in `README.md` without exposing sensitive values.
