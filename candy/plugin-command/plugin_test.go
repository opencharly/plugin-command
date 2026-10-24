package command

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/opencharly/sdk"
	"github.com/opencharly/sdk/kit"
	"github.com/opencharly/spec/spec"
)

// fakeExec is a kit.Executor returning a canned stdout + exit — the in-container command
// RunCapture leg.
type fakeExec struct {
	stdout string
	exit   int
}

func (f *fakeExec) RunCapture(context.Context, string) (string, string, int, error) {
	return f.stdout, "", f.exit, nil
}
func (f *fakeExec) Kind() string { return "container" }

// fakeCC is a minimal kit.CheckContext exercising the command verb's Exec + Mode legs.
type fakeCC struct {
	mode kit.RunMode
	exec kit.Executor
}

func (c *fakeCC) Exec() kit.Executor { return c.exec }
func (c *fakeCC) Mode() kit.RunMode  { return c.mode }
func (c *fakeCC) HTTPDo(context.Context, kit.HTTPRequest) (kit.HTTPResponse, error) {
	return kit.HTTPResponse{}, nil
}
func (c *fakeCC) ResolveEndpoint(context.Context, int) (string, error) { return "", nil }
func (c *fakeCC) ResolveGraphicsEndpoint(context.Context, string) (kit.GraphicsEndpoint, error) {
	return kit.GraphicsEndpoint{}, nil
}
func (c *fakeCC) ResolveImageLabel(context.Context, string) (string, error) { return "", nil }
func (c *fakeCC) DialTimeout() time.Duration                                { return 3 * time.Second }
func (c *fakeCC) Box() string                                               { return "" }
func (c *fakeCC) Instance() string                                          { return "" }
func (c *fakeCC) Distros() []string                                         { return nil }
func (c *fakeCC) AddBackground(int)                                         {}

func runCommandVerb(exit int, input map[string]any, exitStatus *int) kit.Result {
	cc := &fakeCC{mode: kit.ModeLive, exec: &fakeExec{exit: exit}}
	return verb{}.RunVerb(context.Background(), cc, &spec.Op{PluginInput: input, ExitStatus: exitStatus})
}

// TestCommandVerb_ExpectNonZero_PassesOnFailure proves expect_non_zero PASSES when the command
// exits non-zero (whatever the code) — the assertion that had no expression before this field.
func TestCommandVerb_ExpectNonZero_PassesOnFailure(t *testing.T) {
	for _, exit := range []int{1, 2, 127} {
		res := runCommandVerb(exit, map[string]any{"command": "false", "expect_non_zero": true}, nil)
		if res.Status != kit.StatusPass {
			t.Fatalf("expect_non_zero exit=%d: want pass, got %v: %s", exit, res.Status, res.Message)
		}
	}
}

// TestCommandVerb_ExpectNonZero_FailsOnSuccess proves expect_non_zero FAILS when the command
// unexpectedly succeeds (exit 0) — the negative arm that makes the field a real assertion.
func TestCommandVerb_ExpectNonZero_FailsOnSuccess(t *testing.T) {
	res := runCommandVerb(0, map[string]any{"command": "true", "expect_non_zero": true}, nil)
	if res.Status != kit.StatusFail {
		t.Fatalf("expect_non_zero exit=0: want fail, got %v: %s", res.Status, res.Message)
	}
}

// TestCommandVerb_ExactCodeUnaffected proves the historic exit_status behaviour is unchanged when
// expect_non_zero is absent: default asserts 0, and an explicit exit_status asserts that code.
func TestCommandVerb_ExactCodeUnaffected(t *testing.T) {
	if res := runCommandVerb(0, map[string]any{"command": "true"}, nil); res.Status != kit.StatusPass {
		t.Fatalf("default exit_status=0, exit=0: want pass, got %v: %s", res.Status, res.Message)
	}
	if res := runCommandVerb(1, map[string]any{"command": "false"}, nil); res.Status != kit.StatusFail {
		t.Fatalf("default exit_status=0, exit=1: want fail, got %v: %s", res.Status, res.Message)
	}
	want3 := 3
	if res := runCommandVerb(3, map[string]any{"command": "exit 3"}, &want3); res.Status != kit.StatusPass {
		t.Fatalf("exit_status=3, exit=3: want pass, got %v: %s", res.Status, res.Message)
	}
}

// runCommandVerbOut is runCommandVerb's stdout-carrying variant, for matcher assertions
// (Stdout equals/contains/matches) rather than a bare exit-status check.
func runCommandVerbOut(stdout string, exit int, input map[string]any) kit.Result {
	cc := &fakeCC{mode: kit.ModeLive, exec: &fakeExec{stdout: stdout, exit: exit}}
	return verb{}.RunVerb(context.Background(), cc, &spec.Op{PluginInput: input})
}

// TestCommandVerb_StdoutMatchers: exit/stdout matcher evaluation. Relocated from
// charly/checkrun_test.go's TestRunner_CommandVerb (#55 decoupling cone, Batch D) —
// mirrors candy/plugin-port and candy/plugin-http's own test pattern (R3), and reuses this
// file's OWN existing fakeCC/fakeExec/runCommandVerb fixtures (extended with a stdout
// field) rather than introducing a parallel fixture shape.
func TestCommandVerb_StdoutMatchers(t *testing.T) {
	t.Run("exit ok stdout equals", func(t *testing.T) {
		op := map[string]any{"command": "redis-cli ping"}
		res := verb{}.RunVerb(context.Background(), &fakeCC{mode: kit.ModeLive, exec: &fakeExec{stdout: "PONG\n", exit: 0}},
			&spec.Op{PluginInput: op, Stdout: spec.MatcherList{{Op: "equals", Value: "PONG"}}})
		if res.Status != kit.StatusPass {
			t.Errorf("expected pass, got %+v", res)
		}
	})

	t.Run("stdout contains list", func(t *testing.T) {
		op := map[string]any{"command": "status"}
		res := verb{}.RunVerb(context.Background(), &fakeCC{mode: kit.ModeLive, exec: &fakeExec{stdout: "ready ok running", exit: 0}},
			&spec.Op{PluginInput: op, Stdout: spec.MatcherList{{Op: "contains", Value: []any{"ready", "ok"}}}})
		if res.Status != kit.StatusPass {
			t.Errorf("expected pass, got %+v", res)
		}
	})

	t.Run("exit mismatch", func(t *testing.T) {
		res := runCommandVerbOut("", 2, map[string]any{"command": "fail-cmd"})
		if res.Status != kit.StatusFail || !strings.Contains(res.Message, "exit=2") {
			t.Errorf("expected exit failure, got %+v", res)
		}
	})

	t.Run("matches regex", func(t *testing.T) {
		op := map[string]any{"command": "uptime"}
		res := verb{}.RunVerb(context.Background(), &fakeCC{mode: kit.ModeLive, exec: &fakeExec{stdout: "load average: 0.12 0.34 0.56\n", exit: 0}},
			&spec.Op{PluginInput: op, Stdout: spec.MatcherList{{Op: "matches", Value: `load average: [\d.]+`}}})
		if res.Status != kit.StatusPass {
			t.Errorf("expected pass, got %+v", res)
		}
	})
}

// TestCommandVerb_CeilingKillIsReportedAsSuch is the ctx-kill path. A probe the never-hang ceiling
// killed comes back with no exit status (-1: signalled, not returned). Before this fix it fell
// through to the ordinary exit comparison and reported a bare "exit=-1, want 0" — a message that
// names neither the timeout nor how long the probe ran, which is what an RCA needs.
func TestCommandVerb_CeilingKillIsReportedAsSuch(t *testing.T) {
	cc := &fakeCC{mode: kit.ModeLive, exec: &fakeExec{exit: -1}}
	res := verb{}.RunVerb(context.Background(), cc, &spec.Op{PluginInput: map[string]any{"command": "sleep 600"}})

	if res.Status != kit.StatusFail {
		t.Fatalf("status = %v, want fail — a probe that never finished did not demonstrate its property, so it is neither a pass nor a skip", res.Status)
	}
	if strings.Contains(res.Message, "exit=-1, want") {
		t.Errorf("message = %q — still the opaque exit-code report this fix replaces", res.Message)
	}
	for _, want := range []string{"never-hang ceiling", "signalled"} {
		if !strings.Contains(res.Message, want) {
			t.Errorf("message = %q, want it to name %q so the cause is readable without guessing", res.Message, want)
		}
	}
}

// TestCommandVerb_ExpiredContextIsReportedAsCeilingKill covers the other half of the same
// condition: the step ctx expired even though the child reported an ordinary status. The two are
// checked together because they do not always coincide.
func TestCommandVerb_ExpiredContextIsReportedAsCeilingKill(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cc := &fakeCC{mode: kit.ModeLive, exec: &fakeExec{exit: 0}}
	res := verb{}.RunVerb(ctx, cc, &spec.Op{PluginInput: map[string]any{"command": "true"}})

	if res.Status != kit.StatusFail {
		t.Fatalf("status = %v, want fail for a probe whose context expired", res.Status)
	}
	if !strings.Contains(res.Message, "never-hang ceiling") {
		t.Errorf("message = %q, want the ceiling explanation", res.Message)
	}
}

// TestCommandVerb_NormalExitIsUnaffected guards the non-regression: an ordinary non-zero exit must
// still report its code, not be misread as a ceiling kill.
func TestCommandVerb_NormalExitIsUnaffected(t *testing.T) {
	res := runCommandVerb(3, map[string]any{"command": "false"}, nil)
	if res.Status != kit.StatusFail {
		t.Fatalf("status = %v, want fail", res.Status)
	}
	if strings.Contains(res.Message, "never-hang ceiling") {
		t.Errorf("message = %q — an ordinary non-zero exit must not be reported as a timeout", res.Message)
	}
	if !strings.Contains(res.Message, "exit=3") {
		t.Errorf("message = %q, want the actual exit code", res.Message)
	}
}

// TestCommandVerb_HostSideForeground: the host-side foreground path (from_host: true) runs
// the command via os/exec on the host (NOT the container executor) and asserts the stdout
// matcher. Relocated from charly/plugin_command_relocated_test.go's
// TestRelocatedCommandVerb_DispatchesViaKit (the check-role behavior half; the dispatch
// wiring stays in charly).
// recordingExec is a kit.Executor that records the script it was asked to run,
// so a test can assert the step env: modifier was prepended as shell exports.
type recordingExec struct {
	script string
}

func (r *recordingExec) RunCapture(_ context.Context, script string) (string, string, int, error) {
	r.script = script
	return "", "", 0, nil
}
func (r *recordingExec) Kind() string { return "container" }

// TestCommandVerb_StepEnvReachesCommand proves the step's env: modifier (a base
// #Op field) is applied to the executed command as shell exports — the RCA fix
// for the schema-declared-but-never-wired env field. The in-container leg must
// see `export KEY='value'` prepended to the command.
func TestCommandVerb_StepEnvReachesCommand(t *testing.T) {
	rec := &recordingExec{}
	cc := &fakeCC{mode: kit.ModeLive, exec: rec}
	op := &spec.Op{
		PluginInput: map[string]any{"command": "test \"$TEST_ENV_VAR\" = \"hello env\""},
		Env:         map[string]string{"TEST_ENV_VAR": "hello env"},
	}
	res := verb{}.RunVerb(context.Background(), cc, op)
	if res.Status != kit.StatusPass {
		t.Fatalf("env-scoped command: want pass, got %v: %s", res.Status, res.Message)
	}
	if !strings.Contains(rec.script, "export TEST_ENV_VAR='hello env'") {
		t.Fatalf("env-scoped command: script missing the env export, got: %s", rec.script)
	}
}

// TestCommandVerb_NoEnvLeavesCommandUntouched proves a step without env: is
// passed through unchanged (no spurious exports).
func TestCommandVerb_NoEnvLeavesCommandUntouched(t *testing.T) {
	rec := &recordingExec{}
	cc := &fakeCC{mode: kit.ModeLive, exec: rec}
	op := &spec.Op{PluginInput: map[string]any{"command": "echo plain"}}
	res := verb{}.RunVerb(context.Background(), cc, op)
	if res.Status != kit.StatusPass {
		t.Fatalf("plain command: want pass, got %v: %s", res.Status, res.Message)
	}
	if strings.Contains(rec.script, "export ") {
		t.Fatalf("plain command: script unexpectedly has env exports, got: %s", rec.script)
	}
}

func TestCommandVerb_HostSideForeground(t *testing.T) {
	res := verb{}.RunVerb(context.Background(), &fakeCC{mode: kit.ModeLive},
		&spec.Op{PluginInput: map[string]any{"command": "echo charly-cmd-ok", "from_host": true},
			Stdout: spec.MatcherList{{Op: "contains", Value: "charly-cmd-ok"}}})
	if res.Status != kit.StatusPass {
		t.Fatalf("host-foreground: want pass, got %v: %s", res.Status, res.Message)
	}
}

// TestCommandVerb_Background: the host-side background path (from_host: true +
// background: true) starts the command fire-and-forget, registers the PID via
// AddBackground, and reports a backgrounded pid. Relocated from
// charly/plugin_command_relocated_test.go's TestRelocatedCommandVerb_DispatchesViaKit
// (the check-role behavior half; the dispatch wiring stays in charly).
func TestCommandVerb_Background(t *testing.T) {
	res := verb{}.RunVerb(context.Background(), &fakeCC{mode: kit.ModeLive},
		&spec.Op{PluginInput: map[string]any{"command": "sleep 0.2", "from_host": true, "background": true}})
	if res.Status != kit.StatusPass || !strings.Contains(res.Message, "backgrounded") {
		t.Fatalf("background: want pass + backgrounded, got %v: %s", res.Status, res.Message)
	}
}

// TestCommandVerb_PassCarriesStdout: a PASSING command's result carries its captured stdout as
// CapturedValue, so a downstream step can read this step's output (the workflow-IR `$id.stdout`
// / `$id.json` hand-off) instead of the verb discarding it once its matchers have run. Before
// this change the PASS return was a bare kit.Passf that dropped stdout entirely.
func TestCommandVerb_PassCarriesStdout(t *testing.T) {
	const out = "hello from stdout\n"
	res := runCommandVerbOut(out, 0, map[string]any{"command": "echo hello from stdout"})
	if res.Status != kit.StatusPass {
		t.Fatalf("want pass, got %v: %s", res.Status, res.Message)
	}
	if res.CapturedValue != out {
		t.Fatalf("CapturedValue = %q, want the captured stdout %q", res.CapturedValue, out)
	}
	if !strings.Contains(res.Message, "exit=0") {
		t.Errorf("message = %q, want the unchanged exit=N text a matcher may key on", res.Message)
	}
}

// TestCommandVerb_FailCarriesNoStdout: a FAILING command's result carries NO CapturedValue.
// Only a PASS is a value a downstream step may consume — a failed step's output was never
// vouched for, and stashing it would let a consumer act on output the step itself rejected.
func TestCommandVerb_FailCarriesNoStdout(t *testing.T) {
	res := runCommandVerbOut("boom on stdout\n", 1, map[string]any{"command": "false"})
	if res.Status != kit.StatusFail {
		t.Fatalf("want fail, got %v: %s", res.Status, res.Message)
	}
	if res.CapturedValue != "" {
		t.Fatalf("CapturedValue = %q, want empty — a fail path must not stash output", res.CapturedValue)
	}
}

// TestCommandVerb_StdoutTruncatedAtCeiling: stdout over the 1 MiB cap is stored truncated to
// exactly 1 MiB and the truncation is NAMED in Message — a silent truncation would be a value
// the consumer cannot distinguish from the real output, i.e. a fake.
func TestCommandVerb_StdoutTruncatedAtCeiling(t *testing.T) {
	const cap1MiB = 1 << 20
	big := strings.Repeat("x", cap1MiB+123)
	res := runCommandVerbOut(big, 0, map[string]any{"command": "big-output"})
	if res.Status != kit.StatusPass {
		t.Fatalf("want pass, got %v: %s", res.Status, res.Message)
	}
	if len(res.CapturedValue) != cap1MiB {
		t.Fatalf("CapturedValue len = %d, want exactly %d (1 MiB)", len(res.CapturedValue), cap1MiB)
	}
	for _, want := range []string{"exit=0", "truncated", fmt.Sprintf("%d bytes", len(big))} {
		if !strings.Contains(res.Message, want) {
			t.Errorf("message = %q, want it to contain %q so the truncation is visible, not silent", res.Message, want)
		}
	}
}

// TestCommandVerb_StdoutBelowCapUntruncated: the cap is a bound, not a floor — stdout under
// 1 MiB is stored whole and Message keeps the plain exit=N text (no spurious truncation note).
func TestCommandVerb_StdoutBelowCapUntruncated(t *testing.T) {
	const out = "small output\n"
	res := runCommandVerbOut(out, 0, map[string]any{"command": "small"})
	if res.Status != kit.StatusPass {
		t.Fatalf("want pass, got %v: %s", res.Status, res.Message)
	}
	if res.CapturedValue != out {
		t.Fatalf("CapturedValue = %q, want the whole under-cap stdout %q", res.CapturedValue, out)
	}
	if strings.Contains(res.Message, "truncated") {
		t.Errorf("message = %q, want no truncation note for under-cap output", res.Message)
	}
}

// --- the skip channel: skip_exit ---------------------------------------------
//
// The channel exists so a command can say "the phase ran nothing" with the org's
// OWN exit code instead of a private one. The org's code is
// sdk.CheckSkippedExitCode (spec/exitcode: "the phase RAN NOTHING — neither a
// pass nor a failure") — it is read from the contract here, never written as a
// literal, because a second statement of the contract is a second contract (R2).

// TestCommandVerb_SkipExitIsSkipped proves the one thing the channel exists for:
// a command that exits the org's skip code, on a step that DECLARES it, is
// SKIPPED — neither a pass nor a failure.
func TestCommandVerb_SkipExitIsSkipped(t *testing.T) {
	code := sdk.CheckSkippedExitCode
	res := runCommandVerb(code, map[string]any{"command": "exit 3", "skip_exit": code}, nil)
	if res.Status != kit.StatusSkip {
		t.Fatalf("status = %v, want skip — a step that declared skip_exit=%d ran nothing, so it is neither a pass nor a failure: %s",
			res.Status, code, res.Message)
	}
	if !strings.Contains(res.Message, fmt.Sprintf("exit=%d", code)) {
		t.Errorf("message = %q, want it to name the exit code the step ran out with", res.Message)
	}
}

// TestCommandVerb_SkipExitIsOptIn is the NEGATIVE CONTROL, and the reason the
// channel is opt-in: the SAME exit code on a step that does NOT declare it is
// still a FAILURE. A command's exit code is the author's to interpret; reading
// an undeclared 3 as a skip would turn a real failure into a non-failure.
func TestCommandVerb_SkipExitIsOptIn(t *testing.T) {
	res := runCommandVerb(sdk.CheckSkippedExitCode, map[string]any{"command": "exit 3"}, nil)
	if res.Status != kit.StatusFail {
		t.Fatalf("status = %v, want fail — an undeclared exit code is the author's to read, not the verb's", res.Status)
	}
	if !strings.Contains(res.Message, fmt.Sprintf("exit=%d", sdk.CheckSkippedExitCode)) {
		t.Errorf("message = %q, want the actual exit code", res.Message)
	}
}

// TestCommandVerb_SkipExitDoesNotSwallowOtherFailures proves the channel skips
// EXACTLY the declared code: a step that declares skip_exit=3 and whose command
// exits 1 is still a FAILURE.
func TestCommandVerb_SkipExitDoesNotSwallowOtherFailures(t *testing.T) {
	code := sdk.CheckSkippedExitCode
	res := runCommandVerb(1, map[string]any{"command": "exit 1", "skip_exit": code}, nil)
	if res.Status != kit.StatusFail {
		t.Fatalf("status = %v, want fail — only the DECLARED code is a skip", res.Status)
	}
}

// TestCommandVerb_SkipExitRejectsAPrivateCode proves the declaration is
// validated, not trusted: a step that invents its own skip code is refused with
// a message that names the org's code, so "3" cannot mean two things.
func TestCommandVerb_SkipExitRejectsAPrivateCode(t *testing.T) {
	res := runCommandVerb(0, map[string]any{"command": "true", "skip_exit": 7}, nil)
	if res.Status != kit.StatusFail {
		t.Fatalf("status = %v, want fail — a private skip code is an authoring error, not a channel", res.Status)
	}
	for _, want := range []string{"skip_exit=7", fmt.Sprintf("%d", sdk.CheckSkippedExitCode)} {
		if !strings.Contains(res.Message, want) {
			t.Errorf("message = %q, want it to name %q so the cause is readable without guessing", res.Message, want)
		}
	}
}

// TestCommandVerb_SkipExitContradictingExitStatusIsRefused proves a step cannot
// both PASS and SKIP on one exit code: the contradiction is refused loudly
// rather than resolved silently in favour of either reading.
func TestCommandVerb_SkipExitContradictingExitStatusIsRefused(t *testing.T) {
	code := sdk.CheckSkippedExitCode
	res := runCommandVerb(code, map[string]any{"command": "exit 3", "skip_exit": code}, &code)
	if res.Status != kit.StatusFail {
		t.Fatalf("status = %v, want fail — skip_exit and exit_status both declare %d", res.Status, code)
	}
	if !strings.Contains(res.Message, "exit_status") {
		t.Errorf("message = %q, want it to name the contradiction", res.Message)
	}
}

// TestCommandVerb_SkipExitWithBackgroundIsRefused proves a declaration that
// could never take effect is refused instead of being silently inert: a
// backgrounded command has no exit status to interpret.
//
// The assertion names SKIP_EXIT, not "background". The pre-existing background
// refusals ("background: true is host-side only", "background command not
// meaningful under charly check box") also say "background", so an assertion on
// that word alone passes with the guard REMOVED — the test would not discriminate
// the guard it exists to cover. The host-side case is the sharp one: with the
// guard removed, a backgrounded step PASSES ("backgrounded pid=…"), so the
// declaration would be silently inert — exactly what the guard prevents.
func TestCommandVerb_SkipExitWithBackgroundIsRefused(t *testing.T) {
	code := sdk.CheckSkippedExitCode
	cases := []struct {
		name  string
		input map[string]any
	}{
		{"in-container", map[string]any{"command": "true", "skip_exit": code, "background": true}},
		{"host-side", map[string]any{"command": "true", "skip_exit": code, "background": true, "from_host": true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := runCommandVerb(0, tc.input, nil)
			if res.Status != kit.StatusFail {
				t.Fatalf("status = %v, want fail — background: true has no exit status to skip on", res.Status)
			}
			if !strings.Contains(res.Message, "skip_exit") {
				t.Errorf("message = %q, want it to name skip_exit — the pre-existing background refusals say \"background\" too, so that word alone would not discriminate this guard", res.Message)
			}
		})
	}
}

// TestCommandVerb_SkipExitShortCircuitsTheMatchers proves a skipped step's
// stdout/stderr matchers do NOT run: a step that ran nothing has no output to
// assert on, and a matcher that turned the skip back into a failure would
// defeat the channel. The command's stdout is deliberately EMPTY here, which
// no `contains` matcher could accept.
func TestCommandVerb_SkipExitShortCircuitsTheMatchers(t *testing.T) {
	code := sdk.CheckSkippedExitCode
	res := verb{}.RunVerb(context.Background(), &fakeCC{mode: kit.ModeLive, exec: &fakeExec{stdout: "", exit: code}},
		&spec.Op{PluginInput: map[string]any{"command": "exit 3", "skip_exit": code},
			Stdout: spec.MatcherList{{Op: "contains", Value: "never printed"}}})
	if res.Status != kit.StatusSkip {
		t.Fatalf("status = %v, want skip — a step that ran nothing has no output to assert on: %s", res.Status, res.Message)
	}
}

// TestCommandVerb_SkipExitCarriesNoStdout proves a SKIP stashes no value: like
// a failure, a skipped step's output was never vouched for, and a consumer that
// read it would be acting on a value the step itself declined to assert.
func TestCommandVerb_SkipExitCarriesNoStdout(t *testing.T) {
	code := sdk.CheckSkippedExitCode
	res := runCommandVerbOut("output of a step that ran nothing\n", code, map[string]any{"command": "exit 3", "skip_exit": code})
	if res.Status != kit.StatusSkip {
		t.Fatalf("status = %v, want skip", res.Status)
	}
	if res.CapturedValue != "" {
		t.Fatalf("CapturedValue = %q, want empty — a skip must not stash output", res.CapturedValue)
	}
}

// --- the skip channel against a REAL process ---------------------------------
//
// The tests above drive the verb through a canned exit code, which is the file's
// established fixture (and the right shape for the verb's branches). But the
// boundary this verb actually crosses is a PROCESS: the step's command is a shell
// command, and its exit code is what the channel reads. R8 says a boundary that
// crosses a real service runs against that service and is never mocked, so the
// channel is proven here against a real `sh -c` that really exits with the org's
// skip code — the host-side leg (from_host: true) executes through os/exec, so
// no executor is involved at all.

// TestCommandVerb_SkipExitIsSkipped_Live proves the channel against a real
// process: a command that really exits 3, on a step that declares the code, is
// SKIPPED.
func TestCommandVerb_SkipExitIsSkipped_Live(t *testing.T) {
	code := sdk.CheckSkippedExitCode
	res := verb{}.RunVerb(context.Background(), &fakeCC{mode: kit.ModeLive},
		&spec.Op{PluginInput: map[string]any{"command": "exit 3", "from_host": true, "skip_exit": code}})
	if res.Status != kit.StatusSkip {
		t.Fatalf("status = %v, want skip — a real command that exited %d, on a step that declared it, ran nothing: %s",
			res.Status, code, res.Message)
	}
}

// TestCommandVerb_SkipExitIsOptIn_Live is the live NEGATIVE CONTROL: the SAME
// real command, on a step that does NOT declare the code, is still a FAILURE.
// The fake-exec twin of this case is TestCommandVerb_SkipExitIsOptIn; this one
// removes the mock from the comparison, so the opt-in property is proven against
// a process rather than a canned status.
func TestCommandVerb_SkipExitIsOptIn_Live(t *testing.T) {
	res := verb{}.RunVerb(context.Background(), &fakeCC{mode: kit.ModeLive},
		&spec.Op{PluginInput: map[string]any{"command": "exit 3", "from_host": true}})
	if res.Status != kit.StatusFail {
		t.Fatalf("status = %v, want fail — an undeclared exit code is the author's to read, not the verb's", res.Status)
	}
}

// TestCommandVerb_SkipExitPrecedenceOverExpectNonZero proves the precedence the
// schema states: the skip is read FIRST, so a step declaring skip_exit together
// with expect_non_zero means "exit this code → skipped; any other non-zero →
// passed" — a wildcard narrowed by an exact exception, not a contradiction.
// Only an EXACT-code contradiction (skip_exit and exit_status naming the same
// code) is refused, and that is proven by the test above.
func TestCommandVerb_SkipExitPrecedenceOverExpectNonZero(t *testing.T) {
	code := sdk.CheckSkippedExitCode
	skipped := runCommandVerb(code, map[string]any{"command": "exit 3", "skip_exit": code, "expect_non_zero": true}, nil)
	if skipped.Status != kit.StatusSkip {
		t.Fatalf("status = %v, want skip — the skip is read before the exit comparison: %s", skipped.Status, skipped.Message)
	}
	passed := runCommandVerb(1, map[string]any{"command": "exit 1", "skip_exit": code, "expect_non_zero": true}, nil)
	if passed.Status != kit.StatusPass {
		t.Fatalf("status = %v, want pass — any OTHER non-zero still satisfies expect_non_zero: %s", passed.Status, passed.Message)
	}
}
