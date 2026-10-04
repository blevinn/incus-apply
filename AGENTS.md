# AGENTS.md

This repository is a maintained fork of `abiosoft/incus-apply`.

The fork exists to add a small embeddable Go API and native integration points while preserving upstream behavior and keeping the delta easy to review and upstream.

## Workflow

- Make substantive changes on a topic branch and submit them through a pull request.
- Do not merge your own pull requests unless explicitly instructed by the repository owner.
- Do not force-push, rebase, or otherwise rewrite an active review branch unless explicitly requested.
- Prefer additive follow-up commits when responding to review feedback.
- Keep each pull request narrowly scoped and avoid unrelated cleanup.
- State what validation was actually run. Do not claim tests or integration checks that were not executed.

## Upstream compatibility

- Treat `abiosoft/incus-apply` as the upstream source of truth for existing behavior.
- Preserve existing CLI behavior unless the change intentionally modifies it and the PR documents why.
- Keep fork-only changes small and structurally suitable for submission upstream.
- Avoid unnecessary renames, formatting churn, or broad refactors that make upstream synchronization harder.
- When changing code inherited from upstream, prefer changes that can be cherry-picked or proposed upstream without Aginctus-specific assumptions.
- Do not add Aginctus concepts, configuration schema, ownership metadata, or orchestration policy to this repository.

## API direction

The fork should expose the existing reconciliation capabilities as a reusable Go library without requiring callers to invoke the `incus-apply` executable.

- Library APIs should accept structured values, `io.Reader`, or explicit options rather than CLI argument arrays.
- Keep parsing, planning/diffing, and execution separable so callers can inspect a plan before mutation.
- Return structured errors and results rather than process exit codes or command output when designing public APIs.
- Avoid making CLI presentation types part of the public API.
- Preserve deterministic planning and existing reconciliation semantics unless a change is explicitly justified.

## Incus integration

The long-term library path must use the official Incus Go client directly.

- Do not introduce new subprocess calls to `incus`, `incus-apply`, or shell wrappers as a library integration mechanism.
- Existing command-backed behavior inherited from upstream may remain while a native backend is introduced incrementally.
- Keep the backend behind a narrow interface so the existing CLI and native Go implementation can share reconciliation logic during migration.
- Do not weaken ownership, collision, or destructive-operation safeguards for convenience.

## Testing

Before submitting code changes, run the relevant checks when available:

```sh
make fmt
make test
make build
```

Run `make lint` when the local lint toolchain is available.

Changes to Incus behavior should add or update focused tests. Integration-sensitive changes should be exercised against a disposable Incus environment when practical and the PR should describe what was tested.

## Documentation and generated files

- Update documentation when public CLI or Go API behavior changes.
- Keep examples compatible with upstream unless they document fork-specific API usage.
- Regenerate checked-in schema artifacts when their source types change.
- Preserve the Apache-2.0 license and upstream attribution.
