package mcpforge

import (
	"errors"
	"strconv"
	"strings"
	"text/template"
)

// fragment is the shared gate model: a fragment is gated either by a feature
// set (require/any/negate) or by a predicate over the platform context
// (pred); pred takes precedence when set. When require is empty the fragment
// is always included; otherwise all listed features must be present (unless
// any or negate flip the mode, see passes).
type fragment[C FeatureCarrier] struct {
	require []Feature
	pred    Predicate[C]
	any     bool
	negate  bool
}

// passes reports whether the fragment gate matches a context. The predicate
// is a single boolean gate; negate flips it. Feature gates evaluate against
// the feature set extracted from the context.
func (f fragment[C]) passes(fs FeatureSet, ctx C) bool {
	if f.pred != nil {
		return f.negate != f.pred(ctx)
	}
	if len(f.require) == 0 {
		return true
	}
	if f.negate {
		for _, feat := range f.require {
			if fs.Has(feat) {
				return false
			}
		}
		return true
	}
	if f.any {
		for _, feat := range f.require {
			if fs.Has(feat) {
				return true
			}
		}
		return false
	}
	for _, feat := range f.require {
		if !fs.Has(feat) {
			return false
		}
	}
	return true
}

// segment is one piece of a composed description. A segment carries either
// text or a list (a structured ordered/bulleted block) — never both. sep is
// prepended when the output buffer is already non-empty.
type segment[C FeatureCarrier] struct {
	fragment fragment[C]
	text     string
	list     *ListBuilder[C]
	sep      string
}

// DescBuilder composes a description from a static prefix plus
// feature/predicate-gated segments. At resolution time, only segments whose
// gates are satisfied by the context are concatenated; the rest are skipped.
// The builder owns join logic — segment text should not carry leading spaces.
//
// Usage:
//
//	mcpforge.Static[C]("Upload a file and pin it. ...").
//	    When(CFeatFileHostInput, "MUST use `file` when ...").
//	    When(CFeatSourceMint, "Use source.mode=mint ...").
//	    Resolve(ctx)
type DescBuilder[C FeatureCarrier] struct {
	segments []segment[C]
}

// Named separators used as segment joins. They keep joining punctuation out
// of segment text — a segment never begins with whitespace or punctuation;
// the separator declares how it attaches to the preceding segment.
const (
	// SepNone concatenates directly with no separator (a run-on suffix such
	// as a bare terminating period).
	SepNone = ""
	// SepSpace is the default join between two segments (a single space).
	SepSpace = " "
	// SepSentence starts a new sentence after the previous segment.
	SepSentence = ". "
	// SepClause joins a mid-sentence semicolon clause.
	SepClause = "; "
	// SepList joins a serial list item.
	SepList = ", "
	// SepDash joins an em-dash aside.
	SepDash = " — "
	// SepListBlock starts a structured list block on its own line after
	// preceding prose.
	SepListBlock = "\n"
)

// defaultSep is the standard separator between segments.
const defaultSep = SepSpace

// Static starts a DescBuilder with text that is always included.
// As the first segment, it has no separator.
func Static[C FeatureCarrier](text string) DescBuilder[C] {
	return DescBuilder[C]{segments: []segment[C]{{text: text, sep: ""}}}
}

// Clone returns a copy of the builder whose segment slice does not share its
// backing array with the receiver. Composition methods (List/When*/Unless*)
// append to the segment slice, and append() reuses spare capacity, so deriving
// a per-request description from a shared package-level builder without
// cloning can write new segments into the global's backing array and race
// under concurrent callers. Calling Clone before composing guarantees each
// derived builder grows its own array and never mutates the shared global.
func (d DescBuilder[C]) Clone() DescBuilder[C] {
	d.segments = append([]segment[C](nil), d.segments...)
	return d
}

// Static appends text that is always included, regardless of features.
// It can be used mid-chain to insert a fixed segment after conditional ones.
// The default separator (a single space) is prepended when the buffer is
// non-empty; use StaticSep to override.
func (d DescBuilder[C]) Static(text string) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{text: text, sep: defaultSep})}
}

// StaticSep appends always-included text with a custom separator that is
// prepended when the buffer is non-empty. An empty sep means no separator.
func (d DescBuilder[C]) StaticSep(sep, text string) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{text: text, sep: sep})}
}

// When appends text that is included only when the context carries feat.
func (d DescBuilder[C]) When(feat Feature, text string) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{require: []Feature{feat}}, text: text, sep: defaultSep})}
}

// WhenSep is When with a custom separator prepended when the buffer is
// non-empty.
func (d DescBuilder[C]) WhenSep(sep string, feat Feature, text string) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{require: []Feature{feat}}, text: text, sep: sep})}
}

// WhenAll appends text that is included only when the context has every
// feature in feats.
func (d DescBuilder[C]) WhenAll(feats []Feature, text string) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{require: feats}, text: text, sep: defaultSep})}
}

// WhenAny appends text that is included when the context has at least one
// of the listed features. Callers should pass features that are logically
// related (e.g. two relay routes that always co-occur).
func (d DescBuilder[C]) WhenAny(feats []Feature, text string) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{require: feats, any: true}, text: text, sep: defaultSep})}
}

// WhenAnySentence appends text that starts a new sentence (". ") when the
// context has at least one of feats. It mirrors WhenSentence for the
// multi-feature (any) case.
func (d DescBuilder[C]) WhenAnySentence(feats []Feature, text string) DescBuilder[C] {
	return d.WhenAnySep(SepSentence, feats, text)
}

// WhenAnySep is WhenAny with a custom separator prepended when the buffer is
// non-empty.
func (d DescBuilder[C]) WhenAnySep(sep string, feats []Feature, text string) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{require: feats, any: true}, text: text, sep: sep})}
}

// Unless appends text that is included only when the context does NOT have
// feat. Use it for if/else patterns where one segment applies when a
// feature is present and another applies when it is absent.
func (d DescBuilder[C]) Unless(feat Feature, text string) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{require: []Feature{feat}, negate: true}, text: text, sep: defaultSep})}
}

// UnlessSep is Unless with a custom separator prepended when the buffer is
// non-empty.
func (d DescBuilder[C]) UnlessSep(sep string, feat Feature, text string) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{require: []Feature{feat}, negate: true}, text: text, sep: sep})}
}

// ---------------------------------------------------------------------------
// Semantic helpers
//
// These pair each named separator with a word describing the join, so call
// sites read as prose ("when the sink is drop, append a clause") instead of
// spelling out raw separators. The low-level *Sep methods remain for ad-hoc
// joins.
// ---------------------------------------------------------------------------

// WhenSentence appends text that starts a new sentence (". ") when feat is
// present.
func (d DescBuilder[C]) WhenSentence(feat Feature, text string) DescBuilder[C] {
	return d.WhenSep(SepSentence, feat, text)
}

// StaticSentence appends always-included text that starts a new sentence.
func (d DescBuilder[C]) StaticSentence(text string) DescBuilder[C] {
	return d.StaticSep(SepSentence, text)
}

// WhenClause appends a semicolon clause ("; ") when feat is present.
func (d DescBuilder[C]) WhenClause(feat Feature, text string) DescBuilder[C] {
	return d.WhenSep(SepClause, feat, text)
}

// WhenList appends a serial item (", ") when feat is present.
func (d DescBuilder[C]) WhenList(feat Feature, text string) DescBuilder[C] {
	return d.WhenSep(SepList, feat, text)
}

// StaticList appends an always-included serial item (", ").
func (d DescBuilder[C]) StaticList(text string) DescBuilder[C] {
	return d.StaticSep(SepList, text)
}

// WhenDash appends an em-dash aside (" — ") when feat is present.
func (d DescBuilder[C]) WhenDash(feat Feature, text string) DescBuilder[C] {
	return d.WhenSep(SepDash, feat, text)
}

// WhenRun appends text directly with no separator when feat is present. Use
// for run-on suffixes (e.g. a bare terminating period) that must attach the
// end of the previous segment.
func (d DescBuilder[C]) WhenRun(feat Feature, text string) DescBuilder[C] {
	return d.WhenSep(SepNone, feat, text)
}

// UnlessRun appends text directly with no separator when feat is absent.
func (d DescBuilder[C]) UnlessRun(feat Feature, text string) DescBuilder[C] {
	return d.UnlessSep(SepNone, feat, text)
}

// UnlessSentence appends text that starts a new sentence (". ") when feat is
// absent. It mirrors WhenSentence for the negation, so an if/else pair can use
// the same joining language at both branches.
func (d DescBuilder[C]) UnlessSentence(feat Feature, text string) DescBuilder[C] {
	return d.UnlessSep(SepSentence, feat, text)
}

// UnlessClause appends a semicolon clause ("; ") when feat is absent.
func (d DescBuilder[C]) UnlessClause(feat Feature, text string) DescBuilder[C] {
	return d.UnlessSep(SepClause, feat, text)
}

// UnlessList appends a serial item (", ") when feat is absent.
func (d DescBuilder[C]) UnlessList(feat Feature, text string) DescBuilder[C] {
	return d.UnlessSep(SepList, feat, text)
}

// UnlessDash appends an em-dash aside (" — ") when feat is absent.
func (d DescBuilder[C]) UnlessDash(feat Feature, text string) DescBuilder[C] {
	return d.UnlessSep(SepDash, feat, text)
}

// ---------------------------------------------------------------------------
// Predicate gating
//
// These gate a segment on the resolved platform context directly (e.g. one
// specific host or transport) rather than on a feature. They share the
// segment/sep model with the feature gates above, so a context decision
// composes with prose the same way a feature decision does. The consumer
// supplies the predicate constructors (HostIs, TransportIs, IsHosted,
// SurfaceEnabled, ...) so call sites still read as prose; mcpforge stays
// ignorant of what a host or transport is.
// ---------------------------------------------------------------------------

// WhenPred appends text included only when pred passes for the context.
func (d DescBuilder[C]) WhenPred(pred Predicate[C], text string) DescBuilder[C] {
	return d.WhenPredSep(SepSpace, pred, text)
}

// WhenPredSep is WhenPred with a custom separator prepended when the buffer
// is non-empty.
func (d DescBuilder[C]) WhenPredSep(sep string, pred Predicate[C], text string) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{pred: pred}, text: text, sep: sep})}
}

// UnlessPred appends text included only when pred does NOT pass for the
// context.
func (d DescBuilder[C]) UnlessPred(pred Predicate[C], text string) DescBuilder[C] {
	return d.UnlessPredSep(SepSpace, pred, text)
}

// UnlessPredSep is UnlessPred with a custom separator prepended when the
// buffer is non-empty.
func (d DescBuilder[C]) UnlessPredSep(sep string, pred Predicate[C], text string) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{pred: pred, negate: true}, text: text, sep: sep})}
}

// List appends an ordered/bulleted list block. The list's items are rendered
// (and renumbered) at resolve time against the context, so a step a host does
// not support is dropped without leaving a gap. The list block starts on its
// own line (SepListBlock) after any preceding prose.
func (d DescBuilder[C]) List(lb ListBuilder[C]) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{list: &lb, sep: SepListBlock})}
}

// ListWhen appends a list block included only when the context has feat.
func (d DescBuilder[C]) ListWhen(feat Feature, lb ListBuilder[C]) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{require: []Feature{feat}}, list: &lb, sep: SepListBlock})}
}

// ListWhenAll appends a list block included only when the context has every feat.
func (d DescBuilder[C]) ListWhenAll(feats []Feature, lb ListBuilder[C]) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{require: feats}, list: &lb, sep: SepListBlock})}
}

// ListWhenAny appends a list block included only when the context has any
// feat. Use it to surface a chooser only when the profile has more than one
// route to choose among.
func (d DescBuilder[C]) ListWhenAny(feats []Feature, lb ListBuilder[C]) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{require: feats, any: true}, list: &lb, sep: SepListBlock})}
}

// ListUnless appends a list block included only when the context lacks feat.
func (d DescBuilder[C]) ListUnless(feat Feature, lb ListBuilder[C]) DescBuilder[C] {
	return DescBuilder[C]{segments: append(d.segments, segment[C]{fragment: fragment[C]{require: []Feature{feat}, negate: true}, list: &lb, sep: SepListBlock})}
}

// ---------------------------------------------------------------------------
// List block composition
//
// Ordered/bulleted guidance is a structured block, not prose. Inline
// "(1) ... (2) ..." shoehorns a decision sequence into a single string and
// cannot drop a step a host does not support without leaving a numbered gap.
// ListBuilder fixes that: each item is independently gated on the resolved
// context, items that do not apply are dropped, and the survivors are
// renumbered so the block always reads as a contiguous decision order. It is
// the same gate model the rest of the DSL uses, so a list item is the natural
// unit a future decision/logic tree builds on (mirroring GuideDecision).
// ---------------------------------------------------------------------------

// ListMarker is the bullet style a ListBuilder renders.
type ListMarker int

const (
	// ListNumbered renders items as "1. ", "2. ", ... (renumbered after
	// gated-off items are dropped).
	ListNumbered ListMarker = iota
	// ListBulleted renders items as "- " (order matters but position doesn't).
	ListBulleted
)

// listItem is one entry in a ListBuilder. It carries the same gate model as a
// segment (feature set and/or predicate), so individual steps can be included
// or excluded per context.
type listItem[C FeatureCarrier] struct {
	text     string
	fragment fragment[C]
}

// ListBuilder composes a list whose items are independently context-gated.
// It is constructed with mcpforge.List(marker) then chained with Item* calls
// and embedded into a description via DescBuilder.List/ListWhen (or resolved
// directly with Build against a context).
type ListBuilder[C FeatureCarrier] struct {
	items  []listItem[C]
	marker ListMarker
	intro  string // optional lead-in line rendered above the items
}

// List starts a ListBuilder with the given marker style.
func List[C FeatureCarrier](marker ListMarker) ListBuilder[C] { return ListBuilder[C]{marker: marker} }

// Intro sets an optional lead-in line rendered above the first item (e.g.
// "Pick the byte route in this order:").
func (l ListBuilder[C]) Intro(text string) ListBuilder[C] { l.intro = text; return l }

// Item appends an always-included item.
func (l ListBuilder[C]) Item(text string) ListBuilder[C] {
	return l.append(listItem[C]{text: text})
}

// ItemWhen appends an item included only when the context has feat.
func (l ListBuilder[C]) ItemWhen(feat Feature, text string) ListBuilder[C] {
	return l.append(listItem[C]{text: text, fragment: fragment[C]{require: []Feature{feat}}})
}

// ItemUnless appends an item included only when the context lacks feat.
func (l ListBuilder[C]) ItemUnless(feat Feature, text string) ListBuilder[C] {
	return l.append(listItem[C]{text: text, fragment: fragment[C]{require: []Feature{feat}, negate: true}})
}

// ItemWhenAll appends an item included only when the context has every feat.
func (l ListBuilder[C]) ItemWhenAll(feats []Feature, text string) ListBuilder[C] {
	return l.append(listItem[C]{text: text, fragment: fragment[C]{require: feats}})
}

// ItemWhenAny appends an item included only when the context has any feat.
func (l ListBuilder[C]) ItemWhenAny(feats []Feature, text string) ListBuilder[C] {
	return l.append(listItem[C]{text: text, fragment: fragment[C]{require: feats, any: true}})
}

// ItemWhenPred appends an item included only when pred passes for the
// context.
func (l ListBuilder[C]) ItemWhenPred(pred Predicate[C], text string) ListBuilder[C] {
	return l.append(listItem[C]{text: text, fragment: fragment[C]{pred: pred}})
}

// ItemUnlessPred appends an item included only when pred does NOT pass for
// the context.
func (l ListBuilder[C]) ItemUnlessPred(pred Predicate[C], text string) ListBuilder[C] {
	return l.append(listItem[C]{text: text, fragment: fragment[C]{pred: pred, negate: true}})
}

func (l ListBuilder[C]) append(it listItem[C]) ListBuilder[C] {
	l.items = append(l.items, it)
	return l
}

// Build renders the list against a context. Items whose gate does not match
// are dropped and the survivors are renumbered/bulleted. Returns an empty
// string when no items survive. Selection (which items, what number) is
// computed in Go; the line layout (marker + text join) is a static
// text/template so presentation stays declarative.
func (l ListBuilder[C]) Build(ctx C) string {
	fs := ctx.FeatureSet()
	view := listRenderView{Intro: l.intro}
	n := 0
	for _, it := range l.items {
		if !it.fragment.passes(fs, ctx) {
			continue
		}
		n++
		view.Items = append(view.Items, listRenderItem{
			Marker: listMarkerFor(l.marker, n),
			Text:   it.text,
		})
	}
	if len(view.Items) == 0 {
		// Nothing survived gating; a lone intro line over an empty list is
		// noise, so drop the whole block.
		return ""
	}
	var b strings.Builder
	if err := listBlockTemplate.Execute(&b, view); err != nil {
		// The template is static and cannot fail at runtime; the only
		// possible error would be a broken render, which is a programmer
		// error we surface rather than swallow.
		panic(err)
	}
	return b.String()
}

// listMarkerFor formats the per-item marker for the given style and position.
func listMarkerFor(m ListMarker, n int) string {
	if m == ListNumbered {
		return strconv.Itoa(n) + ". "
	}
	return "- "
}

// listRenderView is the data passed to listBlockTemplate: a lead-in line and
// the already-filtered, already-numbered items.
type listRenderView struct {
	Intro string
	Items []listRenderItem
}

// listRenderItem is one rendered line of a list.
type listRenderItem struct {
	Marker string // e.g. "1. " or "- "
	Text   string
}

// listBlockTemplate lays out a list block: an optional intro line, then each
// item as "<marker><text>" on its own line. It owns presentation only; item
// selection and numbering happen before execution.
var listBlockTemplate = template.Must(template.New("listBlock").Parse(
	`{{ if .Intro }}{{ .Intro }}{{ "\n" }}{{ end }}{{ range .Items }}{{ .Marker }}{{ .Text }}{{ "\n" }}{{ end }}`,
))

// ---------------------------------------------------------------------------
// Sentence-block composition
//
// Long guidance (a flow detail, a publish-site paragraph) should be assembled
// from discrete, independently-gateable sentences rather than one monolithic
// string, so a single instruction can be reused or gated without touching the
// whole block. Each sentence is a complete, self-punctuated unit (carries its
// own trailing period); sentences are joined by single spaces, so there is no
// double-punctuation footgun. The gated variants are thin loops over the
// feature gates above, so a block of sentences all apply the same gate.
// ---------------------------------------------------------------------------

// Sentences appends several complete, self-punctuated sentences as discrete
// segments — shorthand for repeated Static() calls that keeps long guidance
// composed instead of collapsed into one giant string.
func (d DescBuilder[C]) Sentences(texts ...string) DescBuilder[C] {
	s := d
	for _, t := range texts {
		s = s.Static(t)
	}
	return s
}

// SentencesWhen appends a block of sentences included only when the context has feat.
func (d DescBuilder[C]) SentencesWhen(feat Feature, texts ...string) DescBuilder[C] {
	s := d
	for _, t := range texts {
		s = s.When(feat, t)
	}
	return s
}

// SentencesUnless appends a block of sentences included only when the context lacks feat.
func (d DescBuilder[C]) SentencesUnless(feat Feature, texts ...string) DescBuilder[C] {
	s := d
	for _, t := range texts {
		s = s.Unless(feat, t)
	}
	return s
}

// SentencesWhenAny appends a block of sentences included only when the context
// has at least one of feats (see WhenAny).
func (d DescBuilder[C]) SentencesWhenAny(feats []Feature, texts ...string) DescBuilder[C] {
	s := d
	for _, t := range texts {
		s = s.WhenAny(feats, t)
	}
	return s
}

// Then splices another builder's segments onto the end, joining with a single
// space. Fragments passed here are treated as complete, self-punctuated units
// (they already end with their own terminator), so the join is a plain space
// rather than an extra sentence separator that would double the punctuation.
func (d DescBuilder[C]) Then(f DescBuilder[C]) DescBuilder[C] {
	return d.splice(f, SepSpace)
}

// ThenSentence splices another builder's segments onto the end, starting a new
// sentence (". ") before the fragment. Use it when the appended fragment is
// itself an un-punctuated clause (no trailing period).
func (d DescBuilder[C]) ThenSentence(f DescBuilder[C]) DescBuilder[C] {
	return d.splice(f, SepSentence)
}

// splice appends a copy of f's segments, overriding f's first segment's
// separator with sep so the boundary join is controlled by the caller.
func (d DescBuilder[C]) splice(f DescBuilder[C], sep string) DescBuilder[C] {
	segs := append([]segment[C](nil), f.segments...)
	if len(segs) > 0 && sep != "" {
		segs[0].sep = sep
	}
	return DescBuilder[C]{segments: append(append([]segment[C](nil), d.segments...), segs...)}
}

// ResolveSegments returns the texts of the segments that match the context, in
// declaration order, without joining separators or text substitutions (e.g.
// {{SOURCES}}). Prefer it over Resolve for structural assertions that must
// hold independent of join punctuation and wording interpolation.
func (d DescBuilder[C]) ResolveSegments(ctx C) []string {
	fs := ctx.FeatureSet()
	var out []string
	for _, s := range d.segments {
		if !s.fragment.passes(fs, ctx) {
			continue
		}
		if s.list != nil {
			if rendered := s.list.Build(ctx); rendered != "" {
				out = append(out, rendered)
			}
			continue
		}
		out = append(out, s.text)
	}
	return out
}

// Resolve concatenates all matching segments in declaration order against the
// given context, inserting each segment's separator when the buffer is
// already non-empty. A list segment renders its block (dropping and
// renumbering gated-off items) against the same context.
func (d DescBuilder[C]) Resolve(ctx C) string {
	fs := ctx.FeatureSet()
	var b strings.Builder
	for _, s := range d.segments {
		if !s.fragment.passes(fs, ctx) {
			continue
		}
		if b.Len() > 0 && s.sep != "" {
			b.WriteString(s.sep)
		}
		if s.list != nil {
			b.WriteString(s.list.Build(ctx))
			continue
		}
		b.WriteString(s.text)
	}
	return b.String()
}

// joinErrs collapses collected validation errors, nil-safe.
func joinErrs(errs []error) error {
	return errors.Join(errs...)
}
