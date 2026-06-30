package dotfiles

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/orivej/go-nix/nix/parser"
)

// BuildNixMap walks scopeDirs and returns a map of category → set of pkgs referenced.
func BuildNixMap(dotfilesRoot string, scopeDirs []string) (map[string]map[string]bool, error) {
	result := make(map[string]map[string]bool)
	for _, scopeRel := range scopeDirs {
		scopeAbs := filepath.Join(dotfilesRoot, scopeRel)
		err := filepath.WalkDir(scopeAbs, nixFileWalker(dotfilesRoot, result))
		if err != nil {
			return result, fmt.Errorf("walk %s: %w", scopeAbs, err)
		}
	}
	return result, nil
}

// nixFileWalker returns a WalkDir callback that populates result with nix package references.
func nixFileWalker(dotfilesRoot string, result map[string]map[string]bool) func(string, os.DirEntry, error) error {
	return func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".nix") {
			return nil
		}
		relPath, _ := filepath.Rel(dotfilesRoot, path)
		cat := categoryFromFile(relPath)
		if cat == "" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			refs := parseNixRefs(string(data))
			if len(refs) > 0 {
				if result[cat] == nil {
					result[cat] = make(map[string]bool)
				}
				for _, r := range refs {
					result[cat][r] = true
				}
			}
		}
		return nil
	}
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
		if s != "" && !isSkip(s) && !nixBuiltins[s] && !nixLibFuncs[s] {
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

var nixBuiltins = map[string]bool{
	"true": true, "false": true, "null": true, "if": true,
	"then": true, "else": true, "let": true, "in": true, "rec": true,
	"with": true, "inherit": true, "import": true, "pkgs": true,
	"lib": true, "config": true, "options": true, "types": true,
}

func isSkip(name string) bool {
	return nixBuiltins[name] || nixLibFuncs[name] || nixSkip[name]
}

var nixLibFuncs = map[string]bool{
	"hostPlatform": true, "buildPlatform": true, "targetPlatform": true,
	"isLinux": true, "isDarwin": true, "isAarch64": true, "isx86_64": true,
	"optionals": true, "optional": true, "mkIf": true, "mkForce": true,
	"mkDefault": true, "mkBefore": true, "mkAfter": true, "mkMerge": true,
	"mkOrder": true, "mkOverride": true, "optionalAttrs": true,
	"inputs": true, "outputs": true, "self": true, "super": true,
	"packages": true, "overlays": true, "overrides": true, "nixpkgs": true,
	"default": true, "example": true, "description": true, "type": true,
	"apply": true, "merge": true,
}

var nixSkip = map[string]bool{
	"stdenv": true, "callPackage": true, "fetchurl": true,
	"fetchFromGitHub": true, "fetchzip": true, "runCommand": true,
	"runCommandCC": true, "symlinkJoin": true, "buildEnv": true,
	"linkFarm": true, "writeShellApplication": true, "writeText": true,
	"writeShellScript": true, "writeShellScriptBin": true,
	"writeScript": true, "writeScriptBin": true, "emptyDirectory": true,
	"buildGoModule": true, "buildNpmPackage": true, "substituteAll": true,
	"substitute": true, "nix-gitignore": true,
	"buildHomeManagerModule": true, "buildNixosModule": true,
	"buildImage": true, "prefetch": true, "importFromBuild": true,
	"makeDesktopItem": true,
	"override": true, "overrideAttrs": true, "extend": true,
	"newScope": true, "recurseIntoAttrs": true, "lowPrio": true, "hiPrio": true,
	"nodePackages": true, "linuxPackages": true, "applePackages": true,
	"gnomeExtensions": true, "nerd-fonts": true, "yaziPlugins": true,
	"qt6Packages": true, "python3Packages": true, "python313Packages": true,
	"kubernetes-helm": true, "kubernetes-helmPlugins": true,
	"typstPackages": true, "vscode-extensions": true, "typstPackages.algo": true, "typstPackages.algorithmic": true,
	"system": true, "path": true, "gcc": true,
	"coreutils": true, "bash": true, "pkg-config": true,
	"findutils": true, "gnused": true, "gnugrep": true,
	"gnutar": true, "gzip": true, "which": true, "diffutils": true,
	"patch": true, "binutils": true,
	"meta": true, "name": true, "version": true, "src": true,
	"enable": true, "package": true,
	"bashInteractive": true,
	"gpg": true, "gpg-agent": true, "ssh": true,
	"dconf": true, "mpris": true, "mcp": true,
	"pipewire": true, "pulseaudio": true,
	"logind": true, "journald": true, "timesyncd": true,
	"resolved": true, "udev": true, "xserver": true,
	"pinentry-qt": true, "psmisc": true,
	"appimageTools": true, "buildFHSEnv": true, "nix-ld": true,
	"getExe": true, "defaultFhsEnvArgs": true,
	"home-manager": true, "smart-enter": true, "full-border": true,
	"vimPlugins": true, "mpvScripts": true, "nushellPlugins": true,
	"agent-skills": true, "cc": true, "appindicator": true, "caffeine": true, "clipboard-indicator": true,
	"dash-to-dock": true, "tiling-assistant": true, "gsconnect": true,
	"kdeconnect": true, "xdg-desktop-portal-gnome": true,
	"gnome-tweaks": true, "xdg-user-dirs": true, "xdg-utils": true,
	"apple-pingfang": true, "zlib": true, "remote_server": true,
	"mpv": true, "fcitx5-chinese-addons": true, "fcitx5-pinyin-zhwiki": true,
}

func categoryFromFile(relPath string) string {
	parts := strings.SplitN(relPath, string(os.PathSeparator), 4)
	if len(parts) < 3 {
		return ""
	}
	switch parts[0] {
	case "home":
		if parts[1] == "base" || parts[1] == "core" {
			return parts[2]
		}
		if parts[1] == "darwin" {
			return "desktop"
		}
		if parts[1] == "nixos" || parts[1] == "extra" {
			return parts[1]
		}
	case "modules":
		if parts[1] == "nixos" && len(parts) >= 3 {
			return parts[2]
		}
		if parts[1] == "darwin" {
			return "desktop"
		}
	}
	return ""
}

func DefaultScope() []string {
	return []string{
		"home/base", "home/core",
		"modules/nixos", "modules/darwin",
		"home/darwin", "home/nixos", "home/extra",
	}
}

func DedupRef(dotfilesRoot string, scopeDirs []string) (map[string][]string, error) {
	pkgFiles := collectPkgFiles(dotfilesRoot, scopeDirs)
	return crossCategoryPkgs(pkgFiles), nil
}

// collectPkgFiles walks nix files and maps each package to the files that reference it.
func collectPkgFiles(dotfilesRoot string, scopeDirs []string) map[string]map[string]bool {
	pkgFiles := make(map[string]map[string]bool)
	for _, scopeRel := range scopeDirs {
		scopeAbs := filepath.Join(dotfilesRoot, scopeRel)
		_ = filepath.WalkDir(scopeAbs, func(path string, d os.DirEntry, err error) error {
			if err == nil {
				collectNixRefsFromFile(dotfilesRoot, path, d, pkgFiles)
			}
			return nil
		})
	}
	return pkgFiles
}

// collectNixRefsFromFile parses a single .nix file and adds its package refs to pkgFiles.
func collectNixRefsFromFile(dotfilesRoot, path string, d os.DirEntry, pkgFiles map[string]map[string]bool) {
	if d.IsDir() || !strings.HasSuffix(d.Name(), ".nix") {
		return
	}
	rel, _ := filepath.Rel(dotfilesRoot, path)
	if categoryFromFile(rel) == "" {
		return
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return
	}
	for _, name := range parseNixRefs(string(data)) {
		if pkgFiles[name] == nil {
			pkgFiles[name] = make(map[string]bool)
		}
		pkgFiles[name][rel] = true
	}
}

// crossCategoryPkgs returns packages referenced in multiple categories.
func crossCategoryPkgs(pkgFiles map[string]map[string]bool) map[string][]string {
	result := make(map[string][]string)
	for pkg, files := range pkgFiles {
		if len(files) <= 1 {
			continue
		}
		cats := make(map[string]bool)
		for f := range files {
			if c := categoryFromFile(f); c != "" {
				cats[c] = true
			}
		}
		if len(cats) <= 1 {
			continue
		}
		for f := range files {
			result[pkg] = append(result[pkg], f)
		}
	}
	return result
}

// LoadSelfBuiltPkgs reads generated.json and returns a set of self-built package names.
func LoadSelfBuiltPkgs(path string) map[string]bool {
	result := make(map[string]bool)
	data, err := os.ReadFile(path)
	if err != nil {
		return result
	}
	var pkgs map[string]any
	if err := json.Unmarshal(data, &pkgs); err != nil {
		return result
	}
	for name := range pkgs {
		result[name] = true
	}
	return result
}
