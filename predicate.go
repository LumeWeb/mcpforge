package mcpforge

// Predicate is a boolean test over the resolved context. Builders use it
// for gates that cannot be expressed as a Feature — a decision specific to
// one host, transport, deployment, or UI surface. The consumer supplies the
// concrete predicate constructors (e.g. HostIs, TransportIs, IsHosted) so
// call sites can still read as prose without mcpforge knowing what a host
// or transport is.
type Predicate[C any] func(C) bool

// Not negates a predicate. Builders use it for the "unless host" style gates.
func Not[C any](p Predicate[C]) Predicate[C] {
	return func(ctx C) bool { return !p(ctx) }
}

// And returns a predicate that passes only when every given predicate
// passes. Builders use it to gate a fragment on a conjunction that no
// single gate expresses (e.g. "this host AND this deployment mode").
func And[C any](preds ...Predicate[C]) Predicate[C] {
	return func(ctx C) bool {
		for _, p := range preds {
			if !p(ctx) {
				return false
			}
		}
		return true
	}
}

// Or returns a predicate that passes when at least one given predicate
// passes. With no predicates it never passes — an empty disjunction is
// unsatisfiable, which keeps "gate on anything" from accidentally meaning
// "always".
func Or[C any](preds ...Predicate[C]) Predicate[C] {
	return func(ctx C) bool {
		for _, p := range preds {
			if p(ctx) {
				return true
			}
		}
		return false
	}
}

// HasFeature returns a predicate that passes when the context carries the
// given feature. It is the generic replacement for direct-feature gates at
// places that take a predicate (rules, guards), so a feature decision does
// not need a separate predicate constructor per call site.
func HasFeature[C FeatureCarrier](f Feature) Predicate[C] {
	return func(ctx C) bool { return ctx.FeatureSet().Has(f) }
}
