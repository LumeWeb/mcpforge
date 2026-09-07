// Package mcpforge provides a context-generic DSL for conditionally
// constructing descriptions, lists, JSON schemas, tool targets, and agent
// guides, with headless resolution over a caller-defined context.
//
// The package is deliberately a pure script layer. It defines builders that
// accept caller-supplied predicates and resolve them against a generic
// context type:
//
//   - It must not import any MCP SDK.
//   - It must not detect MCP hosts or host environments.
//   - It must not contain product-specific content.
//
// All meaning is injected at the call site: the caller decides what the
// context type is, which predicates gate which fragments, and what the
// resulting artifacts are used for. mcpforge only supplies the conditional
// composition mechanics — the tool, prompt, and schema DSL — so that
// consumers can assemble host-specific output without this package knowing
// anything about a particular host or product.
package mcpforge
