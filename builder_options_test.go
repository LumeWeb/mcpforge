package mcpforge

import (
	"testing"

	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/require"
)

// A decision whose branches are all gated off must vanish from the resolved
// guide entirely: nothing should advertise a question with no answer paths.
func TestDecisionWithoutActiveBranchesIsOmitted(t *testing.T) {
	spec := Guide[testCtx]().
		Flow(Flow[testCtx]("publish", "Publish").
			Steps("capabilities").
			Decision(Decision[testCtx]("pick a path?",
				Branch[testCtx]("host file").WhenFeature(featFileHostInput).Steps("upload_file"),
				Branch[testCtx]("raw source").WhenFeature(featSourceMint).Steps("mint_file"),
			)))

	empty := profileWith()
	got := spec.Resolve(empty)
	flow := got.Flows[0]
	require.NotNil(t, flow)
	require.Nil(t, flow.Decision, "a context with no active branch must not see the decision question")
	require.Equal(t, []string{"capabilities"}, flow.Steps)

	// Same shape at the nested level: a surviving branch whose nested
	// decision has no active branches must not carry the dead question.
	inner := Decision[testCtx]("inner?",
		Branch[testCtx]("no").WhenFeature(featMCPApps).Steps("only_on_mcp"),
	)
	nested := Guide[testCtx]().
		Flow(Flow[testCtx]("ship", "Ship").
			Decision(Decision[testCtx]("outer?",
				Branch[testCtx]("go").Steps("capabilities").Next(inner),
			)))
	nestedGot := nested.Resolve(empty)
	require.Nil(t, nestedGot.Flows[0].Decision.Branches[0].Next,
		"a nested decision without active branches must not be advertised")
}

// The same pre-built schema object contributed to two different builders via
// Property must not be mutated by per-contribution options: Description and
// Enum apply only to the built output of the contribution that declared them.
func TestPropertyOptionsDoNotMutateSharedSchema(t *testing.T) {
	shared := &jsonschema.Schema{Type: "object"}

	first := Schema().
		Property("src", shared, Description("first contribution"), Enum("mode_a", "mode_b"))
	prop := func(s *jsonschema.Schema, name string) *jsonschema.Schema {
		v, _ := s.Properties.Get(name)
		return v
	}

	firstDoc := first.Build(featureSet())
	require.Equal(t, "first contribution", prop(firstDoc, "src").Description)
	require.Equal(t, []any{"mode_a", "mode_b"}, prop(firstDoc, "src").Enum)

	second := Schema().
		Property("src", shared)
	secondDoc := second.Build(featureSet())
	require.Empty(t, prop(secondDoc, "src").Description,
		"the first builder's description must not leak through the shared pointer")
	require.Empty(t, prop(secondDoc, "src").Enum,
		"the first builder's enum must not leak through the shared pointer")

	// The caller's object stays pristine across repeated builds.
	require.Empty(t, shared.Description)
	require.Empty(t, shared.Enum)
	again := second.Build(featureSet())
	require.Empty(t, prop(again, "src").Description)
	require.Empty(t, prop(again, "src").Enum)
}
