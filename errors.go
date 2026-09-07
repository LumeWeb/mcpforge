package mcpforge

import "errors"

// Sentinel errors reported by Validate. Target declarations are struct
// literals in consumer code, so mcpforge cannot prevent contradictory
// combinations at construction time; these make the contradictions
// explicit instead of silently dropping or mis-selecting content at
// resolution time.
//
//   - ErrNoTargets: a definition without targets can never materialize,
//     which is almost always an authoring bug rather than an intentional
//     suppression (Hidden exists for that).
//   - ErrAmbiguousDescription: DescFunc overrides Description at resolution
//     time; declaring both makes the static text dead content that reads as
//     real output to a maintainer but never reaches a client.
//   - ErrHiddenWithContent: a hidden target's description/schema content can
//     never be rendered, so carrying it is dead weight that invites drift.
var (
	ErrNoTargets             = errors.New("mcpforge: tool definition declares no targets")
	ErrAmbiguousDescription  = errors.New("mcpforge: target sets both a static description and a description function")
	ErrHiddenWithContent     = errors.New("mcpforge: hidden target also declares description content")
)
