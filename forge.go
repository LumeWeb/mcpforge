package mcpforge

import (
	"encoding/json"
	"strings"
	"text/template"
)

// Category classifies a tool for filtering during discovery. The vocabulary
// belongs to the consumer.
type Category string

// Descriptor is the neutral, handler-free tool descriptor produced by
// ToolForge.Materialize. It carries everything a materialized tool needs to present
// a tool; the consumer maps it onto its own tool model (attaching handlers
// there — this package never owns execution).
type Descriptor struct {
	Name        string
	Title       string
	Description string
	Category    Category
	ReadOnly    bool
	Destructive bool
	// DirectVisible controls whether the tool appears in tools/list
	// (in addition to progressive disclosure).
	DirectVisible bool
	InputSchema   json.RawMessage
	// OutputSchema is the JSON Schema describing the tool's structured
	// output shape. When nil, no outputSchema is declared on the wire.
	OutputSchema   json.RawMessage
	Meta           map[string]any
	SecuritySchemes []SecurityScheme
	SensitiveFlags  []string
}

// Definition is a declarative tool specification whose concrete presentation
// (description, schema, metadata) varies by platform. It carries a set of
// Targets — each a complete, self-contained presentation keyed by feature
// requirements. The forge selects the best-matching target for a platform's
// features at materialization time.
//
// Tools that don't vary by host have exactly one Target with an empty
// Require set. Tools that present differently to different hosts declare
// multiple targets with distinct Require sets.
type Definition[C FeatureCarrier] struct {
	Name     string
	Title    string
	Category Category

	ReadOnly    bool
	Destructive bool
	// DirectVisible controls whether the tool appears in tools/list
	// (in addition to progressive disclosure).
	DirectVisible bool

	// Targets are complete, self-contained presentations of this tool for
	// specific capability contexts. The forge resolves the best-matching
	// target: among all targets whose Require set is fully satisfied by the
	// platform's features, the one with the most required features wins.
	// Ties are broken by declaration order (first wins).
	Targets []Target[C]
}

// Validate reports contradictory or impossible definition declarations (see
// errors.go). It is opt-in authoring feedback; Materialize never calls it.
func (d Definition[C]) Validate() error {
	if len(d.Targets) == 0 {
		return ErrNoTargets
	}
	var errs []error
	for _, t := range d.Targets {
		errs = append(errs, t.Validate())
	}
	return joinErrs(errs)
}

// ToolForge materializes Definitions into concrete Descriptors for a
// specific platform context. It is a pure function of (definitions,
// context) — no side effects, no mutation of inputs.
type ToolForge[C FeatureCarrier] struct {
	defs []Definition[C]
}

// NewToolForge creates a ToolForge from a set of tool definitions.
func NewToolForge[C FeatureCarrier](defs ...Definition[C]) *ToolForge[C] {
	return &ToolForge[C]{defs: defs}
}

// Add appends a tool definition to the forge.
func (f *ToolForge[C]) Add(def Definition[C]) {
	f.defs = append(f.defs, def)
}

// Len returns the number of tool definitions.
func (f *ToolForge[C]) Len() int {
	return len(f.defs)
}

// Materialize resolves Targets for a platform context and produces concrete
// Descriptors. For each Definition:
//  1. The forge finds the best-matching Target (most specific feature
//     match — see ResolveTarget).
//  2. If no target matches or the target is hidden (Visible=false), the
//     tool is excluded.
//  3. Otherwise, a Descriptor is built from the shared identity fields plus
//     the target's presentation fields. The target's static Description is
//     shared as-is here — dynamic DescFunc resolution is the job of
//     ResolveDescription, which surfaces describing per request.
func (f *ToolForge[C]) Materialize(ctx C) []Descriptor {
	result := make([]Descriptor, 0, len(f.defs))
	for _, def := range f.defs {
		target := ResolveTarget(def.Targets, ctx)
		if target == nil || !target.Visible {
			continue
		}
		result = append(result, Descriptor{
			Name:            def.Name,
			Title:           def.Title,
			Description:     target.Description,
			Category:        def.Category,
			ReadOnly:        def.ReadOnly,
			Destructive:     def.Destructive,
			DirectVisible:   def.DirectVisible,
			InputSchema:     target.InputSchema,
			OutputSchema:    target.OutputSchema,
			Meta:            copyMeta(target.Meta),
			SecuritySchemes: target.SecuritySchemes,
			SensitiveFlags:  target.SensitiveFlags,
		})
	}
	return result
}

// RenderInstructions renders project server-instructions against a platform
// context. The instruction TEXT and the gate flags it interpolates are
// product content and stay with the consumer — mcpforge supplies only the
// rendering plumbing: the caller passes a text/template plus a flags
// function that maps the resolved feature set (and the tool count) to the
// template execution data. Like the template, ignoring the execute error is
// deliberate: instructions are advisory prose and a failing render should
// not break server startup.
func RenderInstructions[C FeatureCarrier](ctx C, toolCount int, tmpl *template.Template, flags func(fs FeatureSet, toolCount int) any) string {
	if tmpl == nil {
		return ""
	}
	var buf strings.Builder
	_ = tmpl.Execute(&buf, flags(ctx.FeatureSet(), toolCount))
	return buf.String()
}

// copyMeta returns a shallow copy of m, or nil if m is empty.
func copyMeta(m map[string]any) map[string]any {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
