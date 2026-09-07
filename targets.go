package mcpforge

import "encoding/json"

// SecurityScheme describes a tool's authentication policy to the host. It is
// the transport-neutral form of the per-tool auth declaration: whether a tool
// may run anonymously or requires an OAuth 2.0 access token, and which scopes
// the token must carry.
type SecurityScheme struct {
	// Type is e.g. "noauth" or "oauth2".
	Type string `json:"type"`
	// Scopes enumerates the OAuth scopes the tool requires. Always
	// serialized (even empty) so the oauth2 declaration carries an explicit
	// `scopes` array per the tool-auth contract.
	Scopes []string `json:"scopes"`
}

// Target is a complete presentation of a tool for a specific capability
// context. Every field is self-contained — the forge does not merge or
// compose targets.
//
// Target is the per-context presentation variant carried on tool
// definitions for tools that vary by host environment. The forge selects the
// best-matching target at materialization time based on the context's
// feature set (deterministic: most required features wins, declaration
// order breaks ties — see ResolveTarget).
type Target[C FeatureCarrier] struct {
	// Require lists features that must all be present for this target to be
	// eligible. Empty = matches any platform (universal target).
	Require FeatureSet

	// Visible controls whether the tool appears at all for matching
	// contexts. false = suppress the tool entirely for this context.
	Visible bool

	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
	Meta         map[string]any

	SecuritySchemes []SecurityScheme
	SensitiveFlags  []string

	// DescFunc, when non-nil, is called at resolution time with the platform
	// context to produce a dynamic description. It overrides Description.
	// Used by targets that compose their description from gated segments via
	// DescBuilder.
	DescFunc func(C) string
}

// NewTarget creates a visible Target that requires all given features.
// Among all matching targets, the one with the most required features wins.
// Use it for transport-specific description variants:
//
//	mcpforge.NewTarget[C]("Upload a file...", CFeatFileHostInput, CFeatSourceURL)
func NewTarget[C FeatureCarrier](desc string, features ...Feature) Target[C] {
	return Target[C]{
		Require:     featureSet(features...),
		Visible:     true,
		Description: desc,
	}
}

// FallbackTarget creates a visible Target with no feature requirements.
// It always matches (score 0), so it only wins when no specific target does.
// Every tool's target list should end with a fallback to guarantee
// resolution.
//
//	mcpforge.FallbackTarget[C]("Upload a file...")
func FallbackTarget[C FeatureCarrier](desc string) Target[C] {
	return Target[C]{
		Require:     FeatureSet{},
		Visible:     true,
		Description: desc,
	}
}

// MCPTargets wraps a variadic list of Targets into a slice. Use it when
// declaring a definition's target list for readability:
//
//	Targets: mcpforge.MCPTargets(mcpforge.FallbackTarget[C]("Upload a file..."))
func MCPTargets[C FeatureCarrier](targets ...Target[C]) []Target[C] { return targets }

// HiddenTarget creates an invisible Target that suppresses the tool entirely
// for platforms matching the given features. Useful when a tool should not
// be advertised to certain hosts.
//
//	mcpforge.HiddenTarget[C](CFeatCoLocated)
func HiddenTarget[C FeatureCarrier](features ...Feature) Target[C] {
	return Target[C]{
		Require: featureSet(features...),
		Visible: false,
	}
}

// Validate reports contradictory target declarations (see errors.go). It is
// opt-in authoring feedback; resolution never calls it.
func (t Target[C]) Validate() error {
	var errs []error
	if !t.Visible && (t.Description != "" || t.DescFunc != nil || t.InputSchema != nil || t.OutputSchema != nil) {
		errs = append(errs, ErrHiddenWithContent)
	}
	if t.Visible && t.Description != "" && t.DescFunc != nil {
		errs = append(errs, ErrAmbiguousDescription)
	}
	return joinErrs(errs)
}

// ResolveTarget resolves the best-matching target for a platform context.
// This operates purely on the platform axis (context → features): among all
// targets whose Require set is a subset of the context's features, the one
// with the most required features wins. Ties are broken by declaration order
// (first wins). Returns nil if no target matches.
func ResolveTarget[C FeatureCarrier](targets []Target[C], ctx C) *Target[C] {
	fs := ctx.FeatureSet()
	var best *Target[C]
	bestScore := -1
	for i := range targets {
		t := &targets[i]
		if !fs.HasAll(t.Require) {
			continue
		}
		score := len(t.Require)
		if score > bestScore {
			bestScore = score
			best = t
		}
	}
	return best
}
