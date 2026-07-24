package dotfiles

var nixBuiltins = map[string]bool{
	"true": true, "false": true, "null": true, "if": true,
	"then": true, "else": true, "let": true, "in": true, "rec": true,
	"with": true, "inherit": true, "import": true, "pkgs": true,
	"lib": true, "config": true, "options": true, "types": true,
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
	"formats": true, "toml": true, "ini": true,
}

var nixSkip = map[string]bool{
	// Nix build infrastructure (builders, fetchers, writers)
	"stdenv": true, "callPackage": true, "fetchurl": true,
	"fetchFromGitHub": true, "fetchzip": true, "runCommand": true,
	"runCommandCC": true, "symlinkJoin": true, "buildEnv": true,
	"linkFarm": true, "writeShellApplication": true, "writeText": true,
	"writeShellScript": true, "writeShellScriptBin": true,
	"writeScript": true, "writeScriptBin": true, "emptyDirectory": true,
	"buildGoModule": true, "buildNpmPackage": true, "substituteAll": true,
	"substitute": true, "nix-gitignore": true,
	"buildHomeManagerModule": true, "buildNixosModule": true,
	"prefetch": true, "importFromBuild": true, "makeDesktopItem": true,
	// Override / scope mechanisms
	"override": true, "overrideAttrs": true, "extend": true,
	"newScope": true, "recurseIntoAttrs": true, "lowPrio": true, "hiPrio": true,
	// AppImage / FHS tools
	"appimageTools": true, "buildFHSEnv": true, "getExe": true, "defaultFhsEnvArgs": true,
	// Package set prefixes (children filtered by applyFilters)
	"nodePackages": true, "linuxPackages": true, "applePackages": true,
	"gnomeExtensions": true, "nerd-fonts": true, "yaziPlugins": true,
	"qt6Packages": true, "python3Packages": true, "python313Packages": true,
	"kubernetes-helmPlugins": true,
	"typstPackages":          true, "vscode-extensions": true,
	"vimPlugins": true, "mpvScripts": true, "nushellPlugins": true,
	// Systemd components (part of systemd package, not standalone)
	"logind": true, "journald": true, "timesyncd": true,
	"resolved": true, "udev": true,
	// Derivation attributes / generic names (not packages)
	"meta": true, "name": true, "version": true, "src": true,
	"enable": true, "package": true,
	"system": true, "path": true, "cc": true,
	"remote_server": true, "ssh": true, "home-manager": true,
	// NixOS service/desktop config keys (not packages)
	"desktopManager": true, "displayManager": true, "smartd": true,
	"sudo_local": true,
	// Third-party module program names (not standard NixOS packages)
	"context7": true, "github": true,
	"coreutils":    true,
	"agent-skills": true,
	// Flake inputs / infra modules (not packages)
	"nix-index-database": true,
}

func isSkip(name string) bool {
	return nixBuiltins[name] || nixLibFuncs[name] || nixSkip[name]
}
