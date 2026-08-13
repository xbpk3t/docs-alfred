package dotfiles

import (
	"strings"

	"github.com/orivej/go-nix/nix/parser"
)

// argsetParamSet returns the set of named parameters of the top-level
// function formals (e.g. `{ pkgs, mylib, ... }: ...`).
// nil/empty means the file is not an argset-parameter lambda module.
func argsetParamSet(n *parser.Node, p *parser.Parser) map[string]bool {
	if n == nil || n.Type != parser.FunctionNode {
		return nil
	}
	var argset *parser.Node
	for _, child := range n.Nodes {
		if child.Type == parser.ArgSetNode {
			argset = child
			break
		}
	}
	if argset == nil {
		return nil
	}
	params := make(map[string]bool)
	var walkArg func(*parser.Node)
	walkArg = func(node *parser.Node) {
		if node == nil {
			return
		}
		if node.Type == parser.IDNode {
			if s := nodeStr(node, p); s != "" {
				params[s] = true
			}
			return
		}
		for _, c := range node.Nodes {
			walkArg(c)
		}
	}
	walkArg(argset)
	return params
}

// parseNixRefs extracts package names from .nix file content using go-nix AST.
func parseNixRefs(content string) []string {
	if content == "" {
		return nil
	}
	p, err := parser.ParseString(content)
	if err != nil {
		return nil
	}
	// If the file is a lambda module (the dominant home-manager shape),
	// `with pkgs;` inside its body must not treat arbitrary bare identifiers
	// as package references — those are attribute names / function args.
	argsetParams := argsetParamSet(p.Result, p)
	var pkgs []string
	seen := make(map[string]bool)
	add := func(name string) {
		if name == "" || seen[name] || name == "\"" {
			return
		}
		short := firstSeg(name)
		if short == "" || seen[short] || isSkip(short) || argsetParams[short] {
			return
		}
		seen[short] = true
		pkgs = append(pkgs, short)
	}
	walkAST(p.Result, p, add, false)
	return pkgs
}

func walkAST(n *parser.Node, p *parser.Parser, add func(string), insideWithPkgs bool) {
	if n == nil {
		return
	}

	// `with pkgs; <body>`: mark the entire body as with-pkgs context — bare
	// identifiers there ARE package references (that's the point of `with
	// pkgs;`). The only exception is attrpath binding keys (home.packages /
	// sessionVariables / imports …), which are excluded below by skipping
	// IDNodes that live under an AttrPathNode.
	if n.Type == parser.WithNode && len(n.Nodes) >= 2 && nodeStr(n.Nodes[0], p) == "pkgs" {
		for _, child := range n.Nodes {
			walkAST(child, p, add, true)
		}
		return
	}

	walkSelectPkgs(n, p, add)
	walkBindNode(n, p, add)

	// Inside `with pkgs; [...]`, bare identifiers are packages — except
	// attribute path segments (attrset keys are not packages).
	if insideWithPkgs && n.Type == parser.IDNode && !inAttrPath(n, p) {
		s := nodeStr(n, p)
		if s != "" && !isSkip(s) {
			add(s)
		}
	}

	for _, child := range n.Nodes {
		walkAST(child, p, add, insideWithPkgs)
	}
}

// inAttrPath reports whether the IDNode is a segment of an AttrPathNode
// (i.e. an attribute path like `home.packages` — a key, not a package).
func inAttrPath(n *parser.Node, p *parser.Parser) bool {
	if n == nil || n == p.Result {
		return false
	}
	parent := parentOf(n, p)
	return parent != nil && parent.Type == parser.AttrPathNode
}

// walkSelectPkgs handles `pkgs.XXX` attribute selection nodes.
func walkSelectPkgs(n *parser.Node, p *parser.Parser, add func(string)) {
	if n.Type != parser.SelectNode || len(n.Nodes) < 2 {
		return
	}
	if nodeStr(n.Nodes[0], p) != "pkgs" {
		return
	}
	// Inside the body of a top-level `with pkgs;`, `pkgs.hello` is the same
	// bare `hello` in with-scope — an attrset key, not a package.
	if isWithPkgsContext(p, n) {
		return
	}
	for i := 1; i < len(n.Nodes); i++ {
		for _, s := range collectAttrNames(n.Nodes[i], p) {
			if s != "" && !isSkip(s) {
				add(s)
			}
		}
	}
}

// isWithPkgsContext reports whether node sits inside a `with pkgs; { ... }`
// whose body is an attrset and whose pkgs is bound as a function argument.
// That is the dominant home-manager module shape: bare identifiers there are
// attrset keys / function args, not package references. `with pkgs; [ ... ]`
// (expression position) is left untouched — its bare identifiers ARE packages.
func isWithPkgsContext(p *parser.Parser, n *parser.Node) bool {
	if p == nil || n == nil {
		return false
	}
	params := argsetParamSet(p.Result, p)
	if len(params) == 0 || !params["pkgs"] {
		return false
	}
	for cur := n; cur != nil && cur != p.Result; cur = parentOf(cur, p) {
		if cur.Type == parser.WithNode && len(cur.Nodes) >= 2 && nodeStr(cur.Nodes[0], p) == "pkgs" {
			// body must be an attrset for the attr-key interpretation
			if len(cur.Nodes) >= 2 && cur.Nodes[1].Type == parser.SetNode {
				return true
			}
		}
	}
	return false
}

// parentOf returns the direct parent of n in p.Result's tree.
func parentOf(n *parser.Node, p *parser.Parser) *parser.Node {
	if p == nil || p.Result == nil || n == nil || n == p.Result {
		return nil
	}
	var find func(*parser.Node) *parser.Node
	find = func(cur *parser.Node) *parser.Node {
		if cur == nil {
			return nil
		}
		for _, c := range cur.Nodes {
			if c == n {
				return cur
			}
			if r := find(c); r != nil {
				return r
			}
		}
		return nil
	}
	return find(p.Result)
}

// walkBindNode handles `programs.XXX` / `services.XXX` binding nodes.
// Supports both dotted form (programs.bash.enable = true) and nested form
// (programs = { bash = { enable = true; }; }).
func walkBindNode(n *parser.Node, p *parser.Parser, add func(string)) {
	if n.Type != parser.BindNode || len(n.Nodes) < 1 {
		return
	}
	attrNames := collectAttrNames(n.Nodes[0], p)
	if len(attrNames) < 1 {
		return
	}
	first := attrNames[0]
	if first != "programs" && first != "services" {
		return
	}

	// dotted form: programs.bash.enable = true → attrNames = ["programs","bash","enable"]
	if len(attrNames) >= 2 {
		if target := attrNames[1]; target != "" && !isSkip(target) {
			add(target)
		}
		return
	}

	// nested form: programs = { bash = { ... }; } → attrNames = ["programs"]
	// walk into value node's children to extract inner binding names
	if first == "programs" && len(n.Nodes) >= 2 {
		walkProgramsValue(n.Nodes[1], p, add)
	}
}

// walkProgramsValue extracts binding names from a programs/services attrset value.
// Only extracts bindings whose value is an attrset (e.g. bash = { ... }),
// skipping simple assignments (e.g. enable = true).
func walkProgramsValue(val *parser.Node, p *parser.Parser, add func(string)) {
	if val == nil {
		return
	}
	for _, child := range val.Nodes {
		if child.Type == parser.BindNode && len(child.Nodes) >= 2 {
			if names := collectAttrNames(child.Nodes[0], p); len(names) >= 1 {
				if target := names[0]; target != "" && !isSkip(target) && child.Nodes[1].Type == parser.SetNode {
					add(target)
				}
			}
		}
		// peel non-BindNode wrappers but don't recurse into BindNode values
		if child.Type != parser.BindNode {
			walkProgramsValue(child, p, add)
		}
	}
}

func nodeStr(n *parser.Node, p *parser.Parser) string {
	if n == nil || len(n.Tokens) == 0 {
		return ""
	}
	return p.TokenString(n.Tokens[0])
}

// collectAttrNames walks AttrPathNode children to find all IDNode texts.
// AttrPathNode itself has no tokens, so we must descend into its children.
func collectAttrNames(n *parser.Node, p *parser.Parser) []string {
	if n == nil {
		return nil
	}
	var names []string
	for _, child := range n.Nodes {
		if t := nodeStr(child, p); t != "" {
			names = append(names, t)
		}
		names = append(names, collectAttrNames(child, p)...)
	}
	return names
}

func firstSeg(s string) string {
	if idx := strings.Index(s, "."); idx >= 0 {
		return s[:idx]
	}
	return s
}
