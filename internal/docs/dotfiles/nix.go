package dotfiles

import (
	"strings"

	"github.com/orivej/go-nix/nix/parser"
)

// parseNixRefs extracts package names from .nix file content using go-nix AST.
func parseNixRefs(content string) []string {
	if content == "" {
		return nil
	}
	p, err := parser.ParseString(content)
	if err != nil {
		return nil
	}
	var pkgs []string
	seen := make(map[string]bool)
	add := func(name string) {
		if name == "" || seen[name] || name == "\"" {
			return
		}
		short := firstSeg(name)
		if short == "" || seen[short] || isSkip(short) {
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

	if walkWithPkgs(n, p, add) {
		return
	}
	walkSelectPkgs(n, p, add)
	walkBindNode(n, p, add)

	// Inside `with pkgs; [...]`, bare identifiers are packages
	if insideWithPkgs && n.Type == parser.IDNode {
		s := nodeStr(n, p)
		if s != "" && !isSkip(s) {
			add(s)
		}
	}

	for _, child := range n.Nodes {
		walkAST(child, p, add, insideWithPkgs)
	}
}

// walkWithPkgs handles `with pkgs; <body>` nodes. Returns true if the node was handled.
func walkWithPkgs(n *parser.Node, p *parser.Parser, add func(string)) bool {
	if n.Type != parser.WithNode || len(n.Nodes) < 2 {
		return false
	}
	if nodeStr(n.Nodes[0], p) != "pkgs" {
		return false
	}
	walkAST(n.Nodes[1], p, add, true)
	return true
}

// walkSelectPkgs handles `pkgs.XXX` attribute selection nodes.
func walkSelectPkgs(n *parser.Node, p *parser.Parser, add func(string)) {
	if n.Type != parser.SelectNode || len(n.Nodes) < 2 {
		return
	}
	if nodeStr(n.Nodes[0], p) != "pkgs" {
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
