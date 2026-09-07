package mcpforge

// ---------------------------------------------------------------------------
// Guide wire model
//
// These are the structured types the agent-facing guide tool returns. They
// live here (in the platform-DSL package) so the guide can be composed by
// the same DSL that builds schemas and descriptions, and so consumers only
// alias/map them onto their wire payload instead of maintaining a parallel
// definition.
// ---------------------------------------------------------------------------

// GuideFlow describes one chained flow an agent can drive end-to-end.
// Simple flows use Steps directly. Branching flows use Decision so the agent
// picks the correct path deterministically instead of guessing.
type GuideFlow struct {
	Name     string         `json:"name"`               // flow identifier, e.g. auth
	Title    string         `json:"title"`              // short human label
	Steps    []string       `json:"steps,omitempty"`    // ordered tools (simple flows)
	Detail   string         `json:"detail,omitempty"`   // one-line guidance
	Decision *GuideDecision `json:"decision,omitempty"` // branching flows
}

// GuideDecision models a branching point in a flow. The agent evaluates each
// Branch's When clause and follows the first match.
type GuideDecision struct {
	Question string        `json:"question"` // what to decide
	Branches []GuideBranch `json:"branches"` // ordered branches
}

// GuideBranch is one path through a decision. When is a natural-language
// condition; Steps is the ordered tool chain for that path; Detail is
// guidance; Next allows nested decisions.
type GuideBranch struct {
	When   string         `json:"when"`  // condition for this branch
	Steps  []string       `json:"steps"` // ordered tools for that path
	Detail string         `json:"detail,omitempty"`
	Next   *GuideDecision `json:"next,omitempty"` // nested decision if needed
}

// AgentGuide is the structured guide payload returned by the guide tool.
type AgentGuide struct {
	Summary string      `json:"summary"`
	Flows   []GuideFlow `json:"flows"`
	Rules   []string    `json:"rules,omitempty"` // operational invariants
}

// ---------------------------------------------------------------------------
// Guide DSL
//
// GuideSpec, Flow and Branch compose guide content declaratively from
// feature-gated contributions — the guide twin of SchemaBuilder/DescBuilder.
// Resolving against a platform context emits only the flows/steps/branches
// that platform actually supports, so guide steps can never advertise a tool
// or source mode the resolved tool schema/surface rejects.
// ---------------------------------------------------------------------------

// featureGate holds the when/unless feature pair shared by step, branch and
// rule gating so all apply feature gates identically.
type featureGate struct {
	when   Feature
	unless Feature
}

// allows reports whether a when/unless feature pair matches the context.
// A feature missing from the gate (empty string) never blocks.
func (g featureGate) allows(fs FeatureSet) bool {
	if g.when != "" && !fs.Has(g.when) {
		return false
	}
	if g.unless != "" && fs.Has(g.unless) {
		return false
	}
	return true
}

// stepContribution models one gated step contribution: a feature gate
// and/or predicate plus the names contributed.
type stepContribution[C FeatureCarrier] struct {
	fg    featureGate
	pred  Predicate[C]
	names []string
}

func (s stepContribution[C]) allows(fs FeatureSet, ctx C) bool {
	if s.pred != nil {
		return s.pred(ctx)
	}
	return s.fg.allows(fs)
}

func (s stepContribution[C]) resolve(fs FeatureSet, ctx C) []string {
	if !s.allows(fs, ctx) {
		return nil
	}
	return s.names
}

// resolveSteps flattens an ordered list of gated steps into the tool chain
// active for the context. Both Flow and Branch route through it so step
// gating is defined once.
func resolveSteps[C FeatureCarrier](steps []stepContribution[C], fs FeatureSet, ctx C) []string {
	var out []string
	for _, s := range steps {
		out = append(out, s.resolve(fs, ctx)...)
	}
	return out
}

// addStep appends a single gated contribution to a step chain. Several build
// methods share it to avoid repeating append+return boilerplate.
func addStep[C FeatureCarrier](gates *[]stepContribution[C], g stepContribution[C]) {
	*gates = append(*gates, g)
}

// GuideFlowBuilder declares one flow and resolves it against a context.
type GuideFlowBuilder[C FeatureCarrier] struct {
	name, title string
	gates       []stepContribution[C]
	detail      DescBuilder[C]
	hasDetail   bool
	decision    *GuideDecisionBuilder[C]
}

// Flow starts a flow with the given identifier and human title.
func Flow[C FeatureCarrier](name, title string) *GuideFlowBuilder[C] {
	return &GuideFlowBuilder[C]{name: name, title: title}
}

// Steps appends tool names that are always part of the ordered chain.
func (b *GuideFlowBuilder[C]) Steps(names ...string) *GuideFlowBuilder[C] {
	addStep(&b.gates, stepContribution[C]{names: names})
	return b
}

// StepWhen appends tool names included only when the resolved context has feat.
func (b *GuideFlowBuilder[C]) StepWhen(feat Feature, names ...string) *GuideFlowBuilder[C] {
	addStep(&b.gates, stepContribution[C]{fg: featureGate{when: feat}, names: names})
	return b
}

// StepUnless appends tool names included only when the resolved context lacks feat.
func (b *GuideFlowBuilder[C]) StepUnless(feat Feature, names ...string) *GuideFlowBuilder[C] {
	addStep(&b.gates, stepContribution[C]{fg: featureGate{unless: feat}, names: names})
	return b
}

// StepWhenPred appends tool names included only when pred passes for the
// context.
func (b *GuideFlowBuilder[C]) StepWhenPred(pred Predicate[C], names ...string) *GuideFlowBuilder[C] {
	addStep(&b.gates, stepContribution[C]{pred: pred, names: names})
	return b
}

// StepUnlessPred appends tool names included only when pred does NOT pass
// for the context.
func (b *GuideFlowBuilder[C]) StepUnlessPred(pred Predicate[C], names ...string) *GuideFlowBuilder[C] {
	addStep(&b.gates, stepContribution[C]{pred: Not(pred), names: names})
	return b
}

// Detail sets the flow's guidance, composed as a feature-gated DescBuilder.
func (b *GuideFlowBuilder[C]) Detail(d DescBuilder[C]) *GuideFlowBuilder[C] {
	b.detail = d
	b.hasDetail = true
	return b
}

// Decision attaches a branching decision instead of a flat step chain.
func (b *GuideFlowBuilder[C]) Decision(d *GuideDecisionBuilder[C]) *GuideFlowBuilder[C] {
	b.decision = d
	return b
}

func (b *GuideFlowBuilder[C]) resolve(ctx C, sub func(string) string) GuideFlow {
	fs := ctx.FeatureSet()
	out := GuideFlow{Name: b.name, Title: b.title, Steps: resolveSteps(b.gates, fs, ctx)}
	if b.hasDetail {
		out.Detail = sub(b.detail.Resolve(ctx))
	}
	if b.decision != nil {
		out.Decision = b.decision.resolve(ctx, sub)
	}
	return out
}

// GuideDecisionBuilder declares a branching point in a flow.
type GuideDecisionBuilder[C FeatureCarrier] struct {
	question string
	branches []*GuideBranchBuilder[C]
}

// Decision starts a branching point with the given question and branches.
func Decision[C FeatureCarrier](question string, branches ...*GuideBranchBuilder[C]) *GuideDecisionBuilder[C] {
	return &GuideDecisionBuilder[C]{question: question, branches: branches}
}

func (b *GuideDecisionBuilder[C]) resolve(ctx C, sub func(string) string) *GuideDecision {
	var out []GuideBranch
	for _, br := range b.branches {
		if rb, ok := br.resolve(ctx, sub); ok {
			out = append(out, rb)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return &GuideDecision{Question: b.question, Branches: out}
}

// GuideBranchBuilder declares one path through a GuideDecision.
type GuideBranchBuilder[C FeatureCarrier] struct {
	when       string
	gates      []stepContribution[C]
	detail     DescBuilder[C]
	hasDetail  bool
	next       *GuideDecisionBuilder[C]
	branchGate featureGate // whole-branch feature gate (optional)
	branchPred Predicate[C]
}

// Branch starts a new branch for the given natural-language condition.
func Branch[C FeatureCarrier](when string) *GuideBranchBuilder[C] {
	return &GuideBranchBuilder[C]{when: when}
}

// Steps appends tool names that are always part of the ordered chain.
func (b *GuideBranchBuilder[C]) Steps(names ...string) *GuideBranchBuilder[C] {
	addStep(&b.gates, stepContribution[C]{names: names})
	return b
}

// StepWhen appends tool names included only when the resolved context has feat.
func (b *GuideBranchBuilder[C]) StepWhen(feat Feature, names ...string) *GuideBranchBuilder[C] {
	addStep(&b.gates, stepContribution[C]{fg: featureGate{when: feat}, names: names})
	return b
}

// StepUnless appends tool names included only when the resolved context lacks feat.
func (b *GuideBranchBuilder[C]) StepUnless(feat Feature, names ...string) *GuideBranchBuilder[C] {
	addStep(&b.gates, stepContribution[C]{fg: featureGate{unless: feat}, names: names})
	return b
}

// StepWhenPred appends tool names included only when pred passes for the
// context.
func (b *GuideBranchBuilder[C]) StepWhenPred(pred Predicate[C], names ...string) *GuideBranchBuilder[C] {
	addStep(&b.gates, stepContribution[C]{pred: pred, names: names})
	return b
}

// StepUnlessPred appends tool names included only when pred does NOT pass
// for the context.
func (b *GuideBranchBuilder[C]) StepUnlessPred(pred Predicate[C], names ...string) *GuideBranchBuilder[C] {
	addStep(&b.gates, stepContribution[C]{pred: Not(pred), names: names})
	return b
}

// Detail sets the branch's guidance, composed as a feature-gated DescBuilder.
func (b *GuideBranchBuilder[C]) Detail(d DescBuilder[C]) *GuideBranchBuilder[C] {
	b.detail = d
	b.hasDetail = true
	return b
}

// WhenFeature includes the branch only when the resolved context has feat.
func (b *GuideBranchBuilder[C]) WhenFeature(feat Feature) *GuideBranchBuilder[C] {
	b.branchGate = featureGate{when: feat}
	return b
}

// UnlessFeature includes the branch only when the resolved context lacks feat.
func (b *GuideBranchBuilder[C]) UnlessFeature(feat Feature) *GuideBranchBuilder[C] {
	b.branchGate = featureGate{unless: feat}
	return b
}

// WhenPred includes the branch only when pred passes for the context.
func (b *GuideBranchBuilder[C]) WhenPred(pred Predicate[C]) *GuideBranchBuilder[C] {
	b.branchPred = pred
	return b
}

// UnlessPred includes the branch only when pred does NOT pass for the
// context.
func (b *GuideBranchBuilder[C]) UnlessPred(pred Predicate[C]) *GuideBranchBuilder[C] {
	b.branchPred = Not(pred)
	return b
}

// Next attaches a nested decision.
func (b *GuideBranchBuilder[C]) Next(d *GuideDecisionBuilder[C]) *GuideBranchBuilder[C] {
	b.next = d
	return b
}

// allows reports whether the branch's whole-branch gate passes for the
// context. A platform predicate wins over the feature pair when set.
func (b *GuideBranchBuilder[C]) allows(fs FeatureSet, ctx C) bool {
	if b.branchPred != nil {
		return b.branchPred(ctx)
	}
	return b.branchGate.allows(fs)
}

// resolve materializes the branch for the context, or reports whether it is
// inactive (feature-gated off, or resolved to an empty step chain).
func (b *GuideBranchBuilder[C]) resolve(ctx C, sub func(string) string) (GuideBranch, bool) {
	fs := ctx.FeatureSet()
	if !b.allows(fs, ctx) {
		return GuideBranch{}, false
	}
	steps := resolveSteps(b.gates, fs, ctx)
	if len(steps) == 0 {
		return GuideBranch{}, false
	}
	out := GuideBranch{When: b.when, Steps: steps}
	if b.hasDetail {
		out.Detail = sub(b.detail.Resolve(ctx))
	}
	if b.next != nil {
		out.Next = b.next.resolve(ctx, sub)
	}
	return out, true
}

// gatedRule is one operational rule, optionally gated on a feature or a
// platform predicate (pred wins over the feature pair when set).
type gatedRule[C FeatureCarrier] struct {
	text string
	fg   featureGate
	pred Predicate[C]
}

// GuideSpec is the top-level, context-aware composition of the whole guide:
// a Summary, a set of Rules, and an ordered set of Flows. Resolve against a
// platform context to produce the concrete AgentGuide.
type GuideSpec[C FeatureCarrier] struct {
	summary    DescBuilder[C]
	hasSummary bool
	rules      []gatedRule[C]
	flows      []*GuideFlowBuilder[C]
	substitute func(string) string
}

// Guide starts a new guide specification.
func Guide[C FeatureCarrier]() *GuideSpec[C] { return &GuideSpec[C]{} }

// Summary sets the guide's opening orientation, composed as a DescBuilder.
func (g *GuideSpec[C]) Summary(d DescBuilder[C]) *GuideSpec[C] {
	g.summary = d
	g.hasSummary = true
	return g
}

// Rule adds an always-included operational rule.
func (g *GuideSpec[C]) Rule(text string) *GuideSpec[C] {
	g.rules = append(g.rules, gatedRule[C]{text: text})
	return g
}

// RuleWhen adds a rule included only when the context has feat.
func (g *GuideSpec[C]) RuleWhen(feat Feature, text string) *GuideSpec[C] {
	g.rules = append(g.rules, gatedRule[C]{text: text, fg: featureGate{when: feat}})
	return g
}

// RuleUnless adds a rule included only when the context lacks feat.
func (g *GuideSpec[C]) RuleUnless(feat Feature, text string) *GuideSpec[C] {
	g.rules = append(g.rules, gatedRule[C]{text: text, fg: featureGate{unless: feat}})
	return g
}

// RuleWhenPred adds a rule included only when pred passes for the context.
// It is the escape hatch for gates no single Feature selector expresses,
// e.g. a conjunction built with And.
func (g *GuideSpec[C]) RuleWhenPred(pred Predicate[C], text string) *GuideSpec[C] {
	g.rules = append(g.rules, gatedRule[C]{text: text, pred: pred})
	return g
}

// RuleUnlessPred adds a rule included only when pred does NOT pass for the
// context.
func (g *GuideSpec[C]) RuleUnlessPred(pred Predicate[C], text string) *GuideSpec[C] {
	g.rules = append(g.rules, gatedRule[C]{text: text, pred: Not(pred)})
	return g
}

// Substitute installs a post-resolution text transform applied to every piece
// of rendered content (summary, rules, flow/branch details). It lets platform
// tokens such as the transport source-mode list be interpolated in one place.
func (g *GuideSpec[C]) Substitute(fn func(string) string) *GuideSpec[C] {
	g.substitute = fn
	return g
}

// Flow appends a flow to the guide.
func (g *GuideSpec[C]) Flow(f *GuideFlowBuilder[C]) *GuideSpec[C] {
	g.flows = append(g.flows, f)
	return g
}

// ruleAllows reports whether a rule's gate passes for the context. A
// platform predicate wins over the feature pair when set, matching step and
// branch gating.
func ruleAllows[C FeatureCarrier](r gatedRule[C], fs FeatureSet, ctx C) bool {
	if r.pred != nil {
		return r.pred(ctx)
	}
	return r.fg.allows(fs)
}

// Resolve materializes the AgentGuide for the given platform context.
func (g *GuideSpec[C]) Resolve(ctx C) AgentGuide {
	fs := ctx.FeatureSet()
	sub := g.substitute
	if sub == nil {
		sub = func(s string) string { return s }
	}
	out := AgentGuide{}
	if g.hasSummary {
		out.Summary = sub(g.summary.Resolve(ctx))
	}
	for _, r := range g.rules {
		if ruleAllows(r, fs, ctx) {
			out.Rules = append(out.Rules, sub(r.text))
		}
	}
	for _, f := range g.flows {
		out.Flows = append(out.Flows, f.resolve(ctx, sub))
	}
	return out
}
