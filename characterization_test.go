package mcpforge

import (
	"encoding/json"
	"slices"
	"testing"
	"text/template"

	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/require"
)

// Characterization tests pinning the present behavior of SchemaBuilder,
// Target resolution, ResolveDescription/ResolveInputSchema and ToolForge.

// ---------------------------------------------------------------------------
// SchemaBuilder
// ---------------------------------------------------------------------------

func TestCharacterizationSchemaBuilderGating(t *testing.T) {
	b := Schema().
		StringProperty("name", "always present").
		StringProperty("mint_mode", "gated on mint", When(featSourceMint)).
		BoolProperty("co_located", "excluded on mint", Unless(featSourceMint))

	keys := func(s *jsonschema.Schema) []string {
		return slices.Collect(s.Properties.KeysFromOldest())
	}

	// Mint context: ungated + When feature present + Unless feature present.
	full := b.Build(featureSet(featSourceMint))
	require.Equal(t, []string{"name", "mint_mode"}, keys(full))

	// Non-mint context: When feature absent drops the property, Unless survives.
	minimal := b.Build(featureSet(featSinkLocal))
	require.Equal(t, []string{"name", "co_located"}, keys(minimal))

	// A nil (empty) FeatureSet behaves like a context with no features.
	empty := b.Build(FeatureSet{})
	require.Equal(t, []string{"name", "co_located"}, keys(empty))
}

func TestCharacterizationSchemaBuilderEnumAndDescription(t *testing.T) {
	mode := &jsonschema.Schema{Type: "string"}
	b := Schema().
		Description("upload arguments").
		Property("mode", mode, Enum("mint", "url"), Description("the source mode")).
		StringProperty("name", "a name")

	s := b.Build(featureSet(featSourceMint))
	require.Equal(t, "upload arguments", s.Description)
	require.Equal(t, "object", s.Type)

	modeOut, ok := s.Properties.Get("mode")
	require.True(t, ok)
	require.Equal(t, "string", modeOut.Type)
	require.Equal(t, "the source mode", modeOut.Description)
	require.Equal(t, []any{"mint", "url"}, modeOut.Enum)
}

func TestCharacterizationSchemaBuilderTransform(t *testing.T) {
	prop := &jsonschema.Schema{Type: "string"}
	b := Schema().Property("mode", prop, Transform(func(s *jsonschema.Schema, fs FeatureSet) {
		if fs.Has(featSourceMint) {
			s.Enum = []any{"mint", "url"}
		} else {
			s.Enum = []any{"path"}
		}
	}))

	// Transform runs at Build time against the resolved feature set.
	narrowed, ok := b.Build(featureSet(featSourceMint)).Properties.Get("mode")
	require.True(t, ok)
	require.Equal(t, []any{"mint", "url"}, narrowed.Enum)

	other, ok := b.Build(featureSet(featSourcePath)).Properties.Get("mode")
	require.True(t, ok)
	require.Equal(t, []any{"path"}, other.Enum)

	// Transform mutates the materialized copy per build; the contributed
	// schema itself is never modified.
	require.Empty(t, prop.Enum)
}

func TestCharacterizationSchemaBuilderPropertyOrdering(t *testing.T) {
	b := Schema().
		StringProperty("zeta", "last declared, must stay last").
		StringProperty("alpha", "mid").
		BoolProperty("enabled", "first boolean").
		StringProperty("beta", "gated but in declared order", When(featSinkLocal))

	// Order follows declaration order, not alphabetical order, and gated
	// properties keep their position among the survivors.
	first := b.Build(featureSet(featSourceMint))
	require.Equal(t, []string{"zeta", "alpha", "enabled"}, slices.Collect(first.Properties.KeysFromOldest()))

	second := b.Build(featureSet(featSinkLocal))
	require.Equal(t, []string{"zeta", "alpha", "enabled", "beta"}, slices.Collect(second.Properties.KeysFromOldest()))
}

func TestCharacterizationSchemaBuilderRequired(t *testing.T) {
	b := Schema().
		StringProperty("name", "the name").
		BoolProperty("enabled", "the flag")

	// No required properties → Required stays nil so it is omitted from JSON.
	without := b.Build(featureSet(featSourceMint))
	require.Empty(t, without.Required)
	raw, err := json.Marshal(without)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "required")

	with := b.Required("name").Required("enabled")
	got := with.Build(featureSet(featSourceMint))
	require.Equal(t, []string{"name", "enabled"}, got.Required)
}

func TestCharacterizationSchemaBuilderRawJSONMatchesBuild(t *testing.T) {
	b := Schema().
		Description("upload arguments").
		StringProperty("name", "the name").
		StringProperty("mode", "the mode", When(featSourceMint)).
		Required("name")

	fs := featureSet(featSourceMint)
	want, err := json.Marshal(b.Build(fs))
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(b.RawJSON(fs)))
	require.Contains(t, string(b.RawJSON(fs)), `"mode"`)

	// RawJSON resolves against the feature set the same way Build does.
	minimal, err := json.Marshal(b.Build(featureSet(featSinkLocal)))
	require.NoError(t, err)
	require.JSONEq(t, string(minimal), string(b.RawJSON(featureSet(featSinkLocal))))
	require.NotContains(t, string(b.RawJSON(featureSet(featSinkLocal))), `"mode"`)
}

// ---------------------------------------------------------------------------
// Targets
// ---------------------------------------------------------------------------

func TestCharacterizationTargetConstructors(t *testing.T) {
	specific := NewTarget[testCtx]("mint variant", featSourceMint, featSinkLocal)
	require.True(t, specific.Visible)
	require.Equal(t, featureSet(featSourceMint, featSinkLocal), specific.Require)
	require.Equal(t, "mint variant", specific.Description)

	fallback := FallbackTarget[testCtx]("universal")
	require.True(t, fallback.Visible)
	require.Empty(t, fallback.Require)
	require.Equal(t, "universal", fallback.Description)

	hidden := HiddenTarget[testCtx](featSinkDrop)
	require.False(t, hidden.Visible)
	require.Equal(t, featureSet(featSinkDrop), hidden.Require)

	targets := MCPTargets(specific, fallback, hidden)
	require.Len(t, targets, 3)
}

func TestCharacterizationResolveTargetSelection(t *testing.T) {
	fallback := FallbackTarget[testCtx]("fallback")
	specific := NewTarget[testCtx]("specific", featSourceMint)
	both := NewTarget[testCtx]("both", featSourceMint, featSinkLocal)
	targets := MCPTargets(fallback, specific, both)

	// The most-required-features match wins over less specific ones.
	got := ResolveTarget(targets, profileWith(featSourceMint, featSinkLocal))
	require.Equal(t, "both", got.Description)

	// A context missing one of "both"'s features falls to "specific".
	got = ResolveTarget(targets, profileWith(featSourceMint))
	require.Equal(t, "specific", got.Description)

	// A context matching nothing returns nil.
	require.Nil(t, ResolveTarget(MCPTargets(specific), profileWith(featSinkDrop)))

	// Ties are broken by declaration order: first declared wins.
	first := NewTarget[testCtx]("first", featSourceMint)
	second := NewTarget[testCtx]("second", featSinkLocal)
	tie := ResolveTarget(MCPTargets(first, second), profileWith(featSourceMint, featSinkLocal))
	require.Equal(t, "first", tie.Description)
	reversed := ResolveTarget(MCPTargets(second, first), profileWith(featSourceMint, featSinkLocal))
	require.Equal(t, "second", reversed.Description)
}

func TestCharacterizationResolveTargetHiddenSuppression(t *testing.T) {
	fallback := FallbackTarget[testCtx]("fallback")
	hidden := HiddenTarget[testCtx](featSinkDrop)
	targets := MCPTargets(fallback, hidden)

	// The hidden target matches, so the tool resolves to an invisible target.
	ctx := profileWith(featSinkDrop)
	got := ResolveTarget(targets, ctx)
	require.NotNil(t, got)
	require.False(t, got.Visible)

	// A context that misses the hidden target's features falls back.
	got = ResolveTarget(targets, profileWith(featSourceMint))
	require.True(t, got.Visible)
	require.Equal(t, "fallback", got.Description)
}

func TestCharacterizationTargetValidate(t *testing.T) {
	// A hidden target carrying description content is dead weight.
	hiddenWithDesc := HiddenTarget[testCtx]()
	hiddenWithDesc.Description = "unreachable prose"
	require.ErrorIs(t, hiddenWithDesc.Validate(), ErrHiddenWithContent)

	hiddenWithFunc := HiddenTarget[testCtx]()
	hiddenWithFunc.DescFunc = func(testCtx) string { return "unreachable" }
	require.ErrorIs(t, hiddenWithFunc.Validate(), ErrHiddenWithContent)

	hiddenWithSchema := HiddenTarget[testCtx]()
	hiddenWithSchema.InputSchema = json.RawMessage(`{"type":"object"}`)
	require.ErrorIs(t, hiddenWithSchema.Validate(), ErrHiddenWithContent)

	// Security schemes alone are not content: a bare hidden target is valid.
	require.NoError(t, HiddenTarget[testCtx]().Validate())

	// A visible target declaring both a static description and a DescFunc
	// makes the static text dead content.
	ambiguous := NewTarget[testCtx]("static")
	ambiguous.DescFunc = func(testCtx) string { return "dynamic" }
	require.ErrorIs(t, ambiguous.Validate(), ErrAmbiguousDescription)

	require.NoError(t, NewTarget[testCtx]("static only", featSourceMint).Validate())
	require.NoError(t, FallbackTarget[testCtx]("static only").Validate())
}

// ---------------------------------------------------------------------------
// ResolveDescription / ResolveInputSchema
// ---------------------------------------------------------------------------

func TestCharacterizationResolveDescriptionHitAndMiss(t *testing.T) {
	fallbackSchema := json.RawMessage(`{"type":"object"}`)
	specificSchema := json.RawMessage(`{"type":"object","properties":{"mode":{"type":"string"}}}`)
	targets := MCPTargets(
		Target[testCtx]{
			Require:     featureSet(featSourceMint),
			Visible:     true,
			Description: "mint target",
			InputSchema: specificSchema,
		},
		Target[testCtx]{
			Visible:     true,
			Description: "fallback target",
			InputSchema: fallbackSchema,
		},
	)

	// Hit: the specific target matches.
	desc, ok := ResolveDescription(targets, profileWith(featSourceMint, featSinkLocal))
	require.True(t, ok)
	require.Equal(t, "mint target", desc)

	schema, ok := ResolveInputSchema(targets, profileWith(featSourceMint, featSinkLocal))
	require.True(t, ok)
	require.JSONEq(t, string(specificSchema), string(schema))

	// Hit on the fallback when the specific target's features are missing.
	desc, ok = ResolveDescription(targets, profileWith(featSinkDrop))
	require.True(t, ok)
	require.Equal(t, "fallback target", desc)

	schema, ok = ResolveInputSchema(targets, profileWith(featSinkDrop))
	require.True(t, ok)
	require.JSONEq(t, string(fallbackSchema), string(schema))

	// Miss: no target's Require set is a subset of the context's features.
	noMatch := MCPTargets(Target[testCtx]{Require: featureSet(featSourceMint), Visible: true, Description: "never"})
	desc, ok = ResolveDescription(noMatch, profileWith(featSinkDrop))
	require.False(t, ok)
	require.Empty(t, desc)

	schema, ok = ResolveInputSchema(noMatch, profileWith(featSinkDrop))
	require.False(t, ok)
	require.Nil(t, schema)
}

func TestCharacterizationResolveDescriptionDescFuncOverrides(t *testing.T) {
	target := Target[testCtx]{
		Require:     featureSet(featSourceMint),
		Visible:     true,
		Description: "static text",
		DescFunc:    func(testCtx) string { return "dynamic text" },
	}
	desc, ok := ResolveDescription(MCPTargets(target), profileWith(featSourceMint))
	require.True(t, ok)
	require.Equal(t, "dynamic text", desc)
}

func TestCharacterizationResolveDescriptionHiddenMisses(t *testing.T) {
	targets := MCPTargets(
		FallbackTarget[testCtx]("fallback"),
		HiddenTarget[testCtx](featSinkDrop),
	)

	// A hidden best-match resolves to nothing.
	desc, ok := ResolveDescription(targets, profileWith(featSinkDrop))
	require.False(t, ok)
	require.Empty(t, desc)

	schema, ok := ResolveInputSchema(targets, profileWith(featSinkDrop))
	require.False(t, ok)
	require.Nil(t, schema)

	// A context missing the hidden feature still hits the fallback.
	desc, ok = ResolveDescription(targets, profileWith(featSourceMint))
	require.True(t, ok)
	require.Equal(t, "fallback", desc)
}

// ---------------------------------------------------------------------------
// Forge
// ---------------------------------------------------------------------------

func TestCharacterizationForgeAddLenAndMaterializeFields(t *testing.T) {
	meta := map[string]any{"group": "pins"}
	schemes := []SecurityScheme{{Type: "oauth2", Scopes: []string{"pins:write"}}}
	def := Definition[testCtx]{
		Name:          "upload_file",
		Title:         "Upload File",
		Category:      Category("transfer"),
		ReadOnly:      false,
		Destructive:   true,
		DirectVisible: true,
		Targets: MCPTargets(
			Target[testCtx]{
				Require:         featureSet(featSourceMint),
				Visible:         true,
				Description:     "mint variant",
				InputSchema:     json.RawMessage(`{"type":"object","properties":{"mode":{}}}`),
				OutputSchema:    json.RawMessage(`{"type":"object","properties":{"upload_handle":{}}}`),
				Meta:            meta,
				SecuritySchemes: schemes,
				SensitiveFlags:  []string{"api_key"},
			},
			FallbackTarget[testCtx]("universal variant"),
		),
	}

	forge := NewToolForge[testCtx]()
	require.Equal(t, 0, forge.Len())
	forge.Add(def)
	require.Equal(t, 1, forge.Len())

	got := forge.Materialize(profileWith(featSourceMint))
	require.Len(t, got, 1)
	d := got[0]
	require.Equal(t, "upload_file", d.Name)
	require.Equal(t, "Upload File", d.Title)
	require.Equal(t, "mint variant", d.Description)
	require.Equal(t, Category("transfer"), d.Category)
	require.False(t, d.ReadOnly)
	require.True(t, d.Destructive)
	require.True(t, d.DirectVisible)
	require.JSONEq(t, `{"type":"object","properties":{"mode":{}}}`, string(d.InputSchema))
	require.JSONEq(t, `{"type":"object","properties":{"upload_handle":{}}}`, string(d.OutputSchema))
	require.Equal(t, map[string]any{"group": "pins"}, d.Meta)
	require.Equal(t, schemes, d.SecuritySchemes)
	require.Equal(t, []string{"api_key"}, d.SensitiveFlags)

	// Materialize is a pure function: mutating the target's Meta map after
	// materialization does not leak into the produced descriptor.
	meta["group"] = "changed"
	require.Equal(t, "changed", meta["group"])
	require.Equal(t, map[string]any{"group": "pins"}, d.Meta)
}

func TestCharacterizationForgeMaterializeGating(t *testing.T) {
	// The fallback target has no Meta, so the descriptor's must be nil.
	// FallbackTarget sets an empty non-nil Require, so it matches any context.
	fallbackDef := Definition[testCtx]{
		Name:    "pins_list",
		Title:   "List Pins",
		Targets: MCPTargets(FallbackTarget[testCtx]("universal")),
	}
	forge := NewToolForge(fallbackDef)
	got := forge.Materialize(profileWith(featSinkDrop))
	require.Len(t, got, 1)
	require.Equal(t, "universal", got[0].Description)
	require.Nil(t, got[0].Meta)

	// A hidden target that beats the fallback (most required features)
	// suppresses the tool entirely.
	hiddenDef := Definition[testCtx]{
		Name: "vault_restore",
		Targets: MCPTargets(
			FallbackTarget[testCtx]("fallback"),
			HiddenTarget[testCtx](featSinkDrop, featSinkLocal),
		),
	}
	got = NewToolForge(hiddenDef).Materialize(profileWith(featSinkDrop, featSinkLocal, featSourcePath))
	require.Empty(t, got)

	// The same definition materializes when the hidden target misses.
	got = NewToolForge(hiddenDef).Materialize(profileWith(featSourceMint))
	require.Len(t, got, 1)
	require.Equal(t, "fallback", got[0].Description)

	// A definition whose targets can never match the context is excluded.
	impossible := Definition[testCtx]{
		Name:    "websites_create",
		Targets: MCPTargets(NewTarget[testCtx]("never matches", featSinkDrop)),
	}
	got = NewToolForge(impossible).Materialize(profileWith(featSourceMint))
	require.Empty(t, got)

	// A definition with no targets at all is excluded (authoring bug —
	// surfaced by Validate, silently skipped at materialization).
	got = NewToolForge(Definition[testCtx]{Name: "no_targets"}).Materialize(profileWith(featSourceMint))
	require.Empty(t, got)
}

func TestCharacterizationForgeMaterializeUsesStaticDescription(t *testing.T) {
	// Materialize shares the target's static Description as-is; dynamic
	// DescFunc resolution is the job of ResolveDescription.
	def := Definition[testCtx]{
		Name: "download_file",
		Targets: MCPTargets(Target[testCtx]{
			Visible:     true,
			Description: "static text",
			DescFunc:    func(testCtx) string { return "dynamic text" },
		}),
	}
	got := NewToolForge(def).Materialize(profileWith(featSourceMint))
	require.Len(t, got, 1)
	require.Equal(t, "static text", got[0].Description)
}

func TestCharacterizationDefinitionValidate(t *testing.T) {
	// A definition without targets can never materialize.
	err := Definition[testCtx]{}.Validate()
	require.ErrorIs(t, err, ErrNoTargets)

	// Contradictions on individual targets are propagated.
	hidden := HiddenTarget[testCtx]()
	hidden.Description = "dead"
	ambiguous := NewTarget[testCtx]("static")
	ambiguous.DescFunc = func(testCtx) string { return "dynamic" }
	err = Definition[testCtx]{Targets: MCPTargets(hidden, ambiguous)}.Validate()
	require.ErrorIs(t, err, ErrHiddenWithContent)
	require.ErrorIs(t, err, ErrAmbiguousDescription)

	clean := Definition[testCtx]{Targets: MCPTargets(FallbackTarget[testCtx]("fine"))}
	require.NoError(t, clean.Validate())
}

func TestCharacterizationRenderInstructions(t *testing.T) {
	tmpl := template.Must(template.New("instructions").Parse(
		"{{.ToolCount}} tools{{if .Mint}} mint{{end}}{{if .Path}} path{{end}}"))

	type instrData struct {
		ToolCount int
		Mint      bool
		Path      bool
	}

	// The flags callback receives the resolved feature set and tool count,
	// so per-platform branches render from it.
	got := RenderInstructions(grokHTTP, 3, tmpl, func(fs FeatureSet, toolCount int) any {
		return instrData{ToolCount: toolCount, Mint: fs.Has(featSourceMint), Path: fs.Has(featSourcePath)}
	})
	require.Equal(t, "3 tools mint", got)

	got = RenderInstructions(stdioGeneric, 3, tmpl, func(fs FeatureSet, toolCount int) any {
		return instrData{ToolCount: toolCount, Mint: fs.Has(featSourceMint), Path: fs.Has(featSourcePath)}
	})
	require.Equal(t, "3 tools path", got)

	// A nil template renders to an empty string rather than panicking.
	require.Equal(t, "", RenderInstructions(grokHTTP, 3, nil, func(FeatureSet, int) any { return nil }))
}
