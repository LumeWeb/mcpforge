package mcpforge

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// identity is the substitution stub used by guide resolve assertions.
func identity(s string) string { return s }

func hasStep(t *testing.T, b *GuideFlowBuilder[testCtx], ctx testCtx, name string) bool {
	t.Helper()
	for _, s := range b.resolve(ctx, identity).Steps {
		if s == name {
			return true
		}
	}
	return false
}

func TestFlowStepGating(t *testing.T) {
	const stepName = "upload_status"

	// Always-included steps appear on every profile (mint and non-mint).
	always := Flow[testCtx]("upload", "Upload").Steps("upload_file").Steps("capabilities")
	require.True(t, hasStep(t, always, grokHTTP, "upload_file"))
	require.True(t, hasStep(t, always, stdioGeneric, "upload_file"))

	// StepWhen gates a step to a specific feature.
	whenMint := Flow[testCtx]("upload", "Upload").StepWhen(featSourceMint, stepName)
	require.True(t, hasStep(t, whenMint, grokHTTP, stepName), "mint profile must include the step")
	require.False(t, hasStep(t, whenMint, stdioGeneric, stepName), "non-mint profile must not include the step")

	// StepUnless excludes a step on profiles with the feature.
	unlessMint := Flow[testCtx]("upload", "Upload").StepUnless(featSourceMint, stepName)
	require.False(t, hasStep(t, unlessMint, grokHTTP, stepName), "mint profile must exclude the step")
	require.True(t, hasStep(t, unlessMint, stdioGeneric, stepName), "non-mint profile must include the step")
}

func TestBranchGatingDropsInactiveBranches(t *testing.T) {
	spec := Guide[testCtx]().
		Flow(Flow[testCtx]("publish", "Publish").
			Decision(Decision[testCtx]("pick?",
				Branch[testCtx]("host file").WhenFeature(featFileHostInput).Steps("upload_file", "websites_create"),
				Branch[testCtx]("convert source").UnlessFeature(featFileHostInput).Steps("upload_file", "websites_create"),
			)))

	// FileHostInput profile sees only the host-file branch.
	got := spec.Resolve(openAITunnel)
	pub := got.Flows[0]
	require.NotNil(t, pub.Decision)
	require.Len(t, pub.Decision.Branches, 1, "inactive branch must be dropped")
	require.Equal(t, "host file", pub.Decision.Branches[0].When)

	// The non-FileHostInput profile sees only the convert-source branch.
	got = spec.Resolve(stdioGeneric)
	require.Len(t, got.Flows[0].Decision.Branches, 1)
	require.Equal(t, "convert source", got.Flows[0].Decision.Branches[0].When)
}

func TestHostGating(t *testing.T) {
	// DescBuilder: host-gated prose segment.
	desc := Static[testCtx]("prefix.").
		WhenPred(testHostIs(hostGrok), "grok only.").
		UnlessPred(testHostIs(hostGrok), "not grok.")
	require.Contains(t, desc.Resolve(grokHTTP), "grok only.")
	require.NotContains(t, desc.Resolve(stdioGeneric), "grok only.")
	require.Contains(t, desc.Resolve(stdioGeneric), "not grok.")
	require.NotContains(t, desc.Resolve(grokHTTP), "not grok.")

	// Guide steps: host-gated step names.
	flow := Flow[testCtx]("f", "F").StepWhenPred(testHostIs(hostGrok), "grok_step").Steps("shared_step")
	got := flow.resolve(grokHTTP, identity)
	require.Contains(t, got.Steps, "grok_step")
	require.Contains(t, got.Steps, "shared_step")
	got = flow.resolve(stdioGeneric, identity)
	require.NotContains(t, got.Steps, "grok_step")
	require.Contains(t, got.Steps, "shared_step")

	// Guide branches and rules: whole-branch host gate + host-gated rule.
	spec := Guide[testCtx]().
		RuleWhenPred(testHostIs(hostGrok), "grok rule").
		RuleUnlessPred(testHostIs(hostGrok), "generic rule").
		Flow(Flow[testCtx]("publish", "Publish").
			Decision(Decision[testCtx]("pick?",
				Branch[testCtx]("grok path").WhenPred(testHostIs(hostGrok)).Steps("grok_tool"),
				Branch[testCtx]("generic path").UnlessPred(testHostIs(hostGrok)).Steps("generic_tool"),
			)))
	grok := spec.Resolve(grokHTTP)
	require.Equal(t, []string{"grok rule"}, grok.Rules)
	require.Equal(t, []string{"grok_tool"}, grok.Flows[0].Decision.Branches[0].Steps)
	generic := spec.Resolve(stdioGeneric)
	require.Equal(t, []string{"generic rule"}, generic.Rules)
	require.Equal(t, []string{"generic_tool"}, generic.Flows[0].Decision.Branches[0].Steps)
}

func TestThenSplicesSelfPunctuatedFragments(t *testing.T) {
	a := Static[testCtx]("One sentence.").Static("Second sentence.")
	b := Static[testCtx]("Third sentence.")
	got := a.Then(b).Resolve(stdioGeneric)
	require.Equal(t, "One sentence. Second sentence. Third sentence.", got)
	require.NotContains(t, got, "..", "spliced self-punctuated fragments must not double periods")
}

func TestSubstituteAppliesToAllContent(t *testing.T) {
	spec := Guide[testCtx]().
		Substitute(func(s string) string { return strings.ReplaceAll(s, "{{X}}", "RESOLVED") }).
		Summary(Static[testCtx]("summary {{X}}")).
		Rule("rule {{X}}").
		Flow(Flow[testCtx]("f", "F").Steps("t").Detail(Static[testCtx]("detail {{X}}")))

	got := spec.Resolve(stdioGeneric)
	require.Equal(t, "summary RESOLVED", got.Summary)
	require.Equal(t, []string{"rule RESOLVED"}, got.Rules)
	require.Equal(t, "detail RESOLVED", got.Flows[0].Detail)
}

// TestSurfaceAndHostedGating verifies the surface-domain and deployment gating
// helpers: Surface gates on domain availability, Hosted gates on deployment,
// and the two are independent.
func TestSurfaceAndHostedGating(t *testing.T) {
	hostedFull := testCtx{Surface: fullTestSurface, Hosted: true}
	hostedNoVault := testCtx{Surface: hostedTestSurface, Hosted: true}
	local := testCtx{Surface: fullTestSurface}

	// DescBuilder: surface + hosted segments compose independently.
	desc := Static[testCtx]("P").
		WhenPred(testSurfaceIs(testSurface.vaultOn), "vault on.").
		WhenPred(testHostedIs(true), "hosted.")
	require.Equal(t, "P vault on. hosted.", desc.Resolve(hostedFull))
	require.Equal(t, "P hosted.", desc.Resolve(hostedNoVault))
	require.Equal(t, "P vault on.", desc.Resolve(local))

	// Guide steps: surface-gated step names.
	flow := Flow[testCtx]("f", "F").StepWhenPred(testSurfaceIs(testSurface.vaultOn), "vault_step").Steps("shared")
	require.Contains(t, flow.resolve(hostedFull, identity).Steps, "vault_step")
	require.NotContains(t, flow.resolve(hostedNoVault, identity).Steps, "vault_step")
	require.Contains(t, flow.resolve(hostedNoVault, identity).Steps, "shared")

	// Guide rules + branches: hosted/surface rules and hosted-gated branches.
	spec := Guide[testCtx]().
		RuleWhenPred(testHostedIs(true), "hosted rule").
		RuleUnlessPred(testHostedIs(true), "local rule").
		RuleWhenPred(testSurfaceIs(testSurface.vaultOn), "vault rule").
		RuleUnlessPred(testSurfaceIs(testSurface.vaultOn), "no vault rule").
		Flow(Flow[testCtx]("d", "D").
			Decision(Decision[testCtx]("pick?",
				Branch[testCtx]("hosted").WhenPred(testHostedIs(true)).Steps("hosted_tool"),
				Branch[testCtx]("local").UnlessPred(testHostedIs(true)).Steps("local_tool"))))

	gotten := spec.Resolve(hostedFull)
	require.Equal(t, []string{"hosted rule", "vault rule"}, gotten.Rules)
	require.Equal(t, []string{"hosted_tool"}, gotten.Flows[0].Decision.Branches[0].Steps)

	gottenNV := spec.Resolve(hostedNoVault)
	require.Equal(t, []string{"hosted rule", "no vault rule"}, gottenNV.Rules)

	gottenLocal := spec.Resolve(local)
	require.Equal(t, []string{"local rule", "vault rule"}, gottenLocal.Rules)
	require.Equal(t, []string{"local_tool"}, gottenLocal.Flows[0].Decision.Branches[0].Steps)
}

// TestRuleWhenPredAndGate exercises RuleWhenPred with an And conjunction
// (host + deployment mode), the pattern the Claude Web notice uses so a
// hosted deployment stops special-casing the host.
func TestRuleWhenPredAndGate(t *testing.T) {
	grokLocal := grokHTTP
	grokHosted := cloneFeatures(grokHTTP)
	grokHosted.Hosted = true

	spec := Guide[testCtx]().
		RuleWhenPred(And(testHostIs(hostGrok), Not(testHostedIs(true))), "grok local-only rule").
		RuleWhenPred(And(testHostIs(hostGrok), testHostedIs(true)), "grok hosted rule")

	require.Equal(t, []string{"grok local-only rule"}, spec.Resolve(grokLocal).Rules)
	require.Equal(t, []string{"grok hosted rule"}, spec.Resolve(grokHosted).Rules)
}
