package mcpforge

// Feature is a named capability a host platform may or may not support.
// It functions like a caniuse entry: the forge checks whether the
// connected platform supports a feature to resolve which target variant
// to materialize. The feature vocabulary is owned by the consumer —
// mcpforge only supplies the set mechanics, so a product's features never
// leak into this package.
type Feature string

// FeatureSet is the set of features a platform context supports.
type FeatureSet map[Feature]bool

// Has reports whether the feature set contains f.
func (fs FeatureSet) Has(f Feature) bool {
	return fs[f]
}

// HasAll reports whether the feature set contains every feature in req.
func (fs FeatureSet) HasAll(req FeatureSet) bool {
	for f := range req {
		if !fs[f] {
			return false
		}
	}
	return true
}

// Clone returns a shallow copy of the feature set. Callers that need to
// mutate a context's features (e.g. to overlay runtime flags) MUST clone
// first — the FeatureSet embedded in a shared context may be backed by a
// package-level map.
func (fs FeatureSet) Clone() FeatureSet {
	out := make(FeatureSet, len(fs))
	for f, on := range fs {
		out[f] = on
	}
	return out
}

// FeatureCarrier is the constraint the context type C must satisfy so
// builders can gate fragments on features. It is deliberately a one-method
// interface instead of a concrete struct: the consumer owns the context
// shape (host, transport, deployment, UI quirks) and only promises that a
// feature set can be extracted from it. Everything mcpforge needs to know
// about the platform is that set plus the caller's predicates — this is
// what keeps the DSL free of host detection.
type FeatureCarrier interface {
	FeatureSet() FeatureSet
}

// featureSet builds a FeatureSet from a variadic list of features.
func featureSet(features ...Feature) FeatureSet {
	fs := make(FeatureSet, len(features))
	for _, f := range features {
		fs[f] = true
	}
	return fs
}
