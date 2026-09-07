package mcpforge

import "encoding/json"

// ResolveDescription finds the best-matching target's description for a
// platform context. Among all targets whose Require set is fully satisfied
// by the context's features, the one with the most required features wins
// (ties broken by declaration order). Returns the description and true on
// match; empty string and false if no target matches or the target is
// hidden.
func ResolveDescription[C FeatureCarrier](targets []Target[C], ctx C) (string, bool) {
	target := ResolveTarget(targets, ctx)
	if target == nil || !target.Visible {
		return "", false
	}
	if target.DescFunc != nil {
		return target.DescFunc(ctx), true
	}
	return target.Description, true
}

// ResolveInputSchema finds the best-matching target's input schema for a
// platform context, using the same resolution rules as ResolveDescription.
func ResolveInputSchema[C FeatureCarrier](targets []Target[C], ctx C) (json.RawMessage, bool) {
	target := ResolveTarget(targets, ctx)
	if target == nil || !target.Visible {
		return nil, false
	}
	return target.InputSchema, true
}
