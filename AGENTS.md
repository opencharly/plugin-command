# AGENTS.md — plugin-command

Standalone plugin repo serving the `command:` check verb (`verb:command`,
compiled-in, host-coupled). The plugin is a Go module at
`candy/plugin-command/` (module path
`github.com/opencharly/plugin-command/candy/plugin-command`); the root
`charly.yml` only declares `discover: candy` so the repo is a project and its
candy is scanned.

Canonical files:

- `candy/plugin-command/charly.yml` — the `plugin-command:` candy entity
  (`plugin:` block, `plan:` check).
- `candy/plugin-command/plugin.go` — the verb provider
  (`NewCheckVerb()` / `NewMeta()` / `RunVerb`).
- `candy/plugin-command/schema/command.cue` — the self-contained
  `#CommandInput` (single source for `params/cue_types_gen.go`).
- `candy/plugin-command/params/cue_types_gen.go` — the generated typed input.
- `candy/plugin-command/plugin_test.go` — the provider's Go tests.
- `charly.yml` — the root project manifest (`discover: candy`).
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.
- `README.md` — user overview only; never agent guidance.

## Load these skills first (R0)

- `/charly-check:check` — the declarative check-step surface and the verb
  catalog the `command:` verb is authored through. This candy carries no
  `skill:` entity of its own; the gap is tracked in
  [opencharly/opencharly#291](https://github.com/opencharly/opencharly/issues/291).
- `/charly-internals:plugin` — the plugin authoring reference: the `plugin:`
  block, the `verb` provider class, the per-plugin CUE-schema contract.
- `/charly-internals:git-workflow` — before any git/PR action.

## Build / validate / test

- `go build ./...` in `candy/plugin-command/` — compile the plugin module.
- `go test ./...` in `candy/plugin-command/` — the plugin's Go tests.
- `charly box validate` at the repo root — the structural check (the candy +
  `plugin:` block, CUE schema).
- The merge gate is the **org-wide** `charly/pr-validator` (required check
  `validate / validate`, defined in `opencharly/.github`); this repo has **no**
  per-repo candy gate.
- The R10 consumer is any candy/box plan that authors a `command:` step.

## Modify this repo

- Edit the `plugin-command:` candy entity, the Go source, and
  `schema/command.cue` **together** — the schema is the single source for the
  verb's `params/` struct; regenerate `params/cue_types_gen.go` from it.
- The verb is COMPILED-IN-ONLY: `RunVerb` needs the live kit check context, and
  its `run:` ACT is the dedicated install-task emitCmd branch, not a kit role.
  Keep the matchers on the shared base op (`sdk.MatchAll`), not in this schema.

## Landing

- PR-only. Every change lands through a pull request; the org-required
  `charly/pr-validator` validates the diff and body and arms native auto-merge on
  PASS. Direct pushes to `main` are blocked.
- History lives in `CHANGELOG/` (written by `tag-on-merge` at merge time); the PR
  body IS the changelog.
- The authoritative rulebook is the umbrella `AGENTS.md` in
  `opencharly/opencharly` and `charly/AGENTS.md` in the charly repo. Do not
  restate its rules here.
