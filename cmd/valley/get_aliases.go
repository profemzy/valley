package main

import (
	"strings"

	resourcecommon "valley/internal/resources/common"
)

// aliasRule maps a natural-language phrase (lowercased, trimmed) to a canonical
// resource name and a set of QueryOptions overrides.
type aliasRule struct {
	// phrase is the full lowercased phrase to match (after stripping flag tokens)
	phrase string
	// resource is the canonical Kubernetes resource name to resolve
	resource string
	// applyOpts mutates opts with the appropriate filters for this alias
	applyOpts func(opts *resourcecommon.QueryOptions)
}

var getAliasRules = []aliasRule{
	// ── Pods ─────────────────────────────────────────────────────────────────
	{
		phrase:   "failing pods",
		resource: "pods",
		applyOpts: func(o *resourcecommon.QueryOptions) {
			o.SemanticFilter = "failing"
			o.AllNamespaces = true
		},
	},
	{
		phrase:   "failed pods",
		resource: "pods",
		applyOpts: func(o *resourcecommon.QueryOptions) {
			o.SemanticFilter = "failing"
			o.AllNamespaces = true
		},
	},
	{
		phrase:   "pending pods",
		resource: "pods",
		applyOpts: func(o *resourcecommon.QueryOptions) {
			o.FieldSelector = "status.phase=Pending"
		},
	},
	{
		phrase:   "running pods",
		resource: "pods",
		applyOpts: func(o *resourcecommon.QueryOptions) {
			o.FieldSelector = "status.phase=Running"
		},
	},
	{
		phrase:   "succeeded pods",
		resource: "pods",
		applyOpts: func(o *resourcecommon.QueryOptions) {
			o.FieldSelector = "status.phase=Succeeded"
		},
	},
	// ── Cross-namespace variants ──────────────────────────────────────────────
	{
		phrase:   "failing pods across all namespaces",
		resource: "pods",
		applyOpts: func(o *resourcecommon.QueryOptions) {
			o.SemanticFilter = "failing"
			o.AllNamespaces = true
		},
	},
	{
		phrase:   "pending pods across all namespaces",
		resource: "pods",
		applyOpts: func(o *resourcecommon.QueryOptions) {
			o.FieldSelector = "status.phase=Pending"
			o.AllNamespaces = true
		},
	},
	{
		phrase:   "all failing pods",
		resource: "pods",
		applyOpts: func(o *resourcecommon.QueryOptions) {
			o.SemanticFilter = "failing"
			o.AllNamespaces = true
		},
	},
	// ── Events ───────────────────────────────────────────────────────────────
	{
		phrase:   "warning events",
		resource: "events",
		applyOpts: func(o *resourcecommon.QueryOptions) {
			o.FieldSelector = "type=Warning"
		},
	},
	{
		phrase:   "warnings",
		resource: "events",
		applyOpts: func(o *resourcecommon.QueryOptions) {
			o.FieldSelector = "type=Warning"
		},
	},
	{
		phrase:   "warning events across all namespaces",
		resource: "events",
		applyOpts: func(o *resourcecommon.QueryOptions) {
			o.FieldSelector = "type=Warning"
			o.AllNamespaces = true
		},
	},
}

// AliasResult holds the outcome of a natural-language alias resolution.
type AliasResult struct {
	// Matched is true when a known alias phrase was found.
	Matched bool
	// Resource is the canonical Kubernetes resource name (e.g. "pods").
	Resource string
	// Args is the rewritten args slice with NL words replaced by Resource.
	Args []string
	// FieldSelector is the Kubernetes field selector to inject, if any.
	FieldSelector string
	// SemanticFilter is the in-memory filter keyword to inject, if any.
	SemanticFilter string
	// AllNamespaces is true when the alias implies a cluster-wide query.
	AllNamespaces bool
}

// resolveGetAlias inspects the raw args slice before flag parsing.
//
// It collects all leading non-flag tokens (stopping at the first token that
// begins with "-"), joins them into a phrase, and checks that phrase against
// the known alias table.
//
// Callers must apply AliasResult overrides AFTER fs.Parse so that explicit
// user-supplied flags (-A, --field-selector, etc.) win over alias defaults.
func resolveGetAlias(args []string) AliasResult {
	if len(args) == 0 {
		return AliasResult{}
	}

	// Collect leading non-flag words
	words := []string{}
	flagStart := len(args)
	for i, a := range args {
		if strings.HasPrefix(a, "-") {
			flagStart = i
			break
		}
		words = append(words, a)
	}

	if len(words) == 0 {
		return AliasResult{Resource: args[0], Args: args}
	}

	phrase := strings.ToLower(strings.Join(words, " "))

	// Build a temporary opts to let applyOpts populate fields
	var tmpOpts resourcecommon.QueryOptions
	for _, rule := range getAliasRules {
		if phrase == rule.phrase {
			rule.applyOpts(&tmpOpts)
			newArgs := append([]string{rule.resource}, args[flagStart:]...)
			return AliasResult{
				Matched:        true,
				Resource:       rule.resource,
				Args:           newArgs,
				FieldSelector:  tmpOpts.FieldSelector,
				SemanticFilter: tmpOpts.SemanticFilter,
				AllNamespaces:  tmpOpts.AllNamespaces,
			}
		}
	}

	// No alias matched
	return AliasResult{Resource: args[0], Args: args}
}
