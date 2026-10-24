package command

import (
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// The plugin's OWN schema, validated against the input the verb actually decodes.
//
// The runtime gate for a plugin_input lives in charly core
// (validateAuthoredPluginInput: unify the authored JSON with the served def, then
// Validate(cue.Concrete(true))), and it runs at DISPATCH — a step's plugin_input is
// only checked when the verb is invoked, never by `charly box validate` on the
// manifest. That leaves a hole on THIS side of the boundary: the schema a plugin
// ships and the plugin_input shape the plugin decodes can drift apart, and the
// drift is only discovered when a step runs.
//
// #CommandInput is SELF-CONTAINED (every field is a bare primitive referencing no
// base def — the schema header states this), so it compiles standalone and can be
// validated here against the plugin's own embedded source, with no base and no
// charly binary. The def is a CUE DEFINITION, so it is closed: an unknown field is
// rejected — which is what makes the "accepts skip_exit" case a real assertion
// rather than a tautology.

// commandInputDef compiles the plugin's embedded schema and returns its
// #CommandInput definition, the exact def the runtime gate unifies authored
// plugin_input against.
func commandInputDef(t *testing.T) cue.Value {
	t.Helper()
	src, err := schemaFS.ReadFile("schema/command.cue")
	if err != nil {
		t.Fatalf("read embedded schema: %v", err)
	}
	ctx := cuecontext.New()
	v := ctx.CompileBytes(src, cue.Filename("schema/command.cue"))
	if v.Err() != nil {
		t.Fatalf("compile schema: %v", v.Err())
	}
	def := v.LookupPath(cue.ParsePath("#CommandInput"))
	if def.Err() != nil {
		t.Fatalf("lookup #CommandInput: %v", def.Err())
	}
	return def
}

// validateCommandInput mirrors charly core's validateAuthoredPluginInput: unify
// the authored JSON with the def, then validate requiring concreteness.
func validateCommandInput(t *testing.T, def cue.Value, inputJSON string) error {
	t.Helper()
	ctx := cuecontext.New()
	in := ctx.CompileString(inputJSON)
	if in.Err() != nil {
		t.Fatalf("compile input: %v", in.Err())
	}
	return def.Unify(in).Validate(cue.Concrete(true))
}

// TestCommandInputSchema_AcceptsSkipExit proves the shipped schema carries the
// skip_exit field the verb reads. Without the schema change the unify fails with
// "field not allowed", so this test can fail.
func TestCommandInputSchema_AcceptsSkipExit(t *testing.T) {
	def := commandInputDef(t)
	for _, input := range []string{
		`{"command":"true","skip_exit":3}`,
		`{"command":"scripts/check-foo.mjs","skip_exit":3,"from_host":true}`,
		`{"command":"true"}`,
	} {
		if err := validateCommandInput(t, def, input); err != nil {
			t.Errorf("schema rejected valid input %s: %v", input, err)
		}
	}
}

// TestCommandInputSchema_RejectsUnknownField is the NEGATIVE CONTROL for the
// acceptance test: the def is closed, so a typo'd field is refused. If this test
// ever passes with a typo'd field, the acceptance test above is a tautology and
// proves nothing.
func TestCommandInputSchema_RejectsUnknownField(t *testing.T) {
	def := commandInputDef(t)
	if err := validateCommandInput(t, def, `{"command":"true","skip_exi":3}`); err == nil {
		t.Error("schema accepted an unknown field (skip_exi) — the def is not closed, so the acceptance test proves nothing")
	}
}

// TestCommandInputSchema_RejectsWrongType proves the field is typed as the org's
// integer exit code, not a string that would decode into nothing.
func TestCommandInputSchema_RejectsWrongType(t *testing.T) {
	def := commandInputDef(t)
	if err := validateCommandInput(t, def, `{"command":"true","skip_exit":"3"}`); err == nil {
		t.Error("schema accepted skip_exit as a string")
	}
}

// TestCommandInputSchema_RequiresCommand proves the plugin's own required field
// still holds — the schema is not opened up by the new optional field.
func TestCommandInputSchema_RequiresCommand(t *testing.T) {
	def := commandInputDef(t)
	if err := validateCommandInput(t, def, `{"skip_exit":3}`); err == nil {
		t.Error("schema accepted an input with no command")
	}
}
