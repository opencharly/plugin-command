# plugin-command

The `command:` check verb for OpenCharly — run a shell command in-container,
host-side, or backgrounded, and assert its exit status, stdout, or stderr.

`command:` is the most-used declarative check verb: any candy or box plan can
bake a `command:` step into its `check:` / `run:` blocks. The verb's
implementation lives in this out-of-tree module (`verb:command`), compiled into
charly as a host-coupled kit provider — `RunVerb` needs the live check engine
(the venue executor for in-container runs, `os/exec` under `charly check live`).

## What it provides

| Capability | Surface |
|---|---|
| `verb:command` | the declarative `command:` check step |

## The verb

An authored `command: <shell string>` step (scalar sugar) or
`command: {command: …, in_container: false}` (map form). The command-exclusive
fields live in the plugin's own `#CommandInput` (`schema/command.cue`); the
shared `exit_status` / `stdout` / `stderr` matchers ride the base step op.

| Field | Meaning |
|---|---|
| `command` | the shell command to run (the verb discriminator; multi-line OK) |
| `in_container` | run via `podman exec` (default `true`) or host-side `sh -c` (`false`) |
| `from_host` | force host-side execution (equivalent to `in_container: false`) |
| `background` | host-side fire-and-forget; plan teardown reaps the PID via SIGTERM |
| `expect_non_zero` | assert the command FAILS (exit code != 0); mutually exclusive with `exit_status` |

## How to use it

Compose the plugin candy in a box or check bed's `candy:` list:

```yaml
- '@github.com/opencharly/plugin-command/candy/plugin-command:<tag>'
```

Then author the verb in a plan:

```yaml
- check: the service answers on its port
  command: curl -fsS http://127.0.0.1:8080/health
  stdout:
    - contains: ok
```

## Layout

- `candy/plugin-command/` — the plugin module: `plugin.go` (the provider +
  `NewCheckVerb()` / `NewMeta()`), `schema/command.cue` (the self-contained
  `#CommandInput`), `params/cue_types_gen.go`, `plugin_test.go`,
  `cmd/serve/main.go`.
- `charly.yml` — the root project manifest (`discover: candy`).
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.

## Related

- Owning skill: `/charly-check:check` — the check verb catalog. This candy
  carries no `skill:` entity of its own; the gap is tracked in
  [opencharly/opencharly#291](https://github.com/opencharly/opencharly/issues/291).
- `/charly-internals:plugin` — the plugin/provider model.
- [`opencharly/charly`](https://github.com/opencharly/charly) — the charly CLI.
