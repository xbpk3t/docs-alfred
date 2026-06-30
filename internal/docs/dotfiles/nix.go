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
func walkBindNode(n *parser.Node, p *parser.Parser, add func(string)) {
	if n.Type != parser.BindNode || len(n.Nodes) < 1 {
		return
	}
	attrNames := collectAttrNames(n.Nodes[0], p)
	if len(attrNames) < 2 {
		return
	}
	first := attrNames[0]
	if first == "programs" || first == "services" {
		if target := attrNames[1]; target != "" && !isSkip(target) {
			add(target)
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
