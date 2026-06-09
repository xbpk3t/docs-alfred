# Go Code Simplification Spec

## Context

The Go module contains several command-line tools and shared service packages. A review of the current codebase found that the project already uses useful libraries for hard edge cases, including:

- `mvdan.cc/xurls/v2` for URL extraction.
- `github.com/PuerkitoBio/goquery` for HTML link/title extraction.
- `codeberg.org/readeck/go-readability/v2` for article extraction.
- `github.com/asticode/go-astisub` for subtitle parsing.
- `github.com/gabriel-vasile/mimetype` for content type detection.
- `github.com/bmatcuk/doublestar/v4` for recursive globbing.
- `github.com/go-viper/mapstructure/v2` for map-to-struct decoding.

The main simplification opportunity is not broad dependency addition. It is consolidating repeated parsing, URL cleanup, and decoding behavior around dependencies that are already present.

`go test ./...` passes before this spec.

## Goals

- Reduce hand-written parsing and cleanup code where an existing dependency already owns the hard cases.
- Centralize URL extraction, URL cleaning, and URL reference tracking so inbox processing and transcript discovery behave consistently.
- Replace small hand-written language tag matching with `golang.org/x/text/language` where subtitle selection benefits from BCP-47 matching.
- Reduce manual `map[string]any` traversal in `service/ghdata` by using direct typed YAML decoding or `mapstructure`.
- Keep behavioral changes narrow and covered by existing or focused new tests.

## Non-Goals

- Do not add a crawler/browser library such as `colly`, `chromedp`, or `rod`.
- Do not replace domain-specific finance parser rules with broad permissive parsers.
- Do not introduce a new validation framework for existing YAML checks.
- Do not rewrite CLI command structure or public command flags.
- Do not remove dependencies as part of this work unless a later `go mod tidy` proves they are unused after implementation.

## Proposed Phases

### Phase 1: Centralize URL Extraction

Create reusable helpers in `pkg/urlutil` for text and Markdown URL extraction.

Suggested API shape:

```go
type URLRef struct {
    URL        string
    Normalized string
    Start      int
    End        int
}

type ExtractOptions struct {
    BaseURL      string
    Markdown     bool
    BareURLs     bool
    HTTPOnly     bool
    Normalize    bool
    Deduplicate  bool
    TranscriptOnly bool
}

func ExtractURLRefs(text string, opts ExtractOptions) []URLRef
func CleanHTTPURL(raw string) string
func NormalizeSet(urls []string) map[string]bool
```

Implementation notes:

- Use `xurls.Strict()` for inbox-style HTTP URL extraction.
- Use `xurls.Relaxed()` only where scheme-less or loose text URLs are intended, such as transcript descriptions.
- Use `goquery` for HTML anchor extraction when the input is HTML or feed description/content HTML.
- Keep source byte offsets only for original-line operations such as inbox flushing. If HTML extraction cannot preserve offsets, return URL-only refs from that path.
- Keep current trailing punctuation trimming rules, but move them into one helper.
- Keep `urlutil.Normalize` as the canonical dedupe key.

Primary call sites:

- `service/wiki/write.go`: replace `urlRegex`, `markdownLinkRegex`, `extractInboxLineURLRefs`, `cleanInboxURL`, and `normalizedURLSet` internals with `pkg/urlutil` helpers while preserving line removal behavior.
- `rss2nl/transcript/provider.go`: replace `extractTranscriptLinksFromText`, `normalizeCandidateURL`, and duplicate filtering internals with shared helpers plus transcript-specific filtering.
- `service/wiki/fetch.go`: replace direct `xurls.Strict()` link counting with shared extraction or a small `urlutil.CountURLs` helper.

Tests:

- Preserve existing `service/wiki/write_test.go` and `rss2nl/transcript/provider_test.go` expectations.
- Add focused tests in `pkg/urlutil` for:
  - Markdown links with titles.
  - Bare URLs with trailing punctuation.
  - Angle-bracket URLs.
  - Duplicate normalized URLs.
  - Relative URL resolution against a base URL.
  - Transcript-only filtering for `.txt`, `.vtt`, `.srt`, `.json`, and `transcript` paths.

Acceptance criteria:

- Inbox parsing and flushing behavior remains unchanged for existing tests.
- Transcript link extraction behavior remains unchanged for existing tests.
- URL cleanup rules have one implementation outside tests.
- `go test ./pkg/urlutil ./service/wiki ./rss2nl/transcript` passes.

### Phase 2: Use BCP-47 Language Matching for Subtitles

Replace hand-written subtitle language matching in `service/wiki/fetch.go` with `golang.org/x/text/language`.

Target functions:

- `videoSubtitleLangs`
- `pickSubtitleFromMap`
- `langMatches`
- `normalizeLang`
- `metadataLooksChinese`

Implementation notes:

- Parse known preferred tags into `language.Tag` values.
- Use `language.NewMatcher` to rank available subtitle language keys.
- Retain explicit compatibility for legacy or non-standard keys used by media platforms, including `zh-Hans`, `zh-CN`, `zh-Hant`, `zh-TW`, `cmn`, `zho`, and `chi`.
- Preserve deterministic fallback by sorting subtitle language keys before matching.
- Keep Han character detection as a fallback for missing or unreliable metadata language values.

Tests:

- Add or extend `service/wiki/fetch_test.go` cases for:
  - Chinese metadata preferring Chinese subtitles.
  - English metadata preferring English subtitles.
  - Region/script variants matching a generic `zh` preference.
  - Unknown language falling back deterministically.

Acceptance criteria:

- Subtitle selection is deterministic.
- Existing media fetch tests pass.
- Matching logic no longer relies only on string prefix comparisons.

### Phase 3: Simplify `ghdata` Map Decoding

Reduce manual `map[string]any` traversal in `service/ghdata/types.go`.

Preferred approach:

- First evaluate direct `goccy/go-yaml` decoding into typed structs with `yaml` tags.
- Use `mapstructure` only if direct typed YAML decoding cannot preserve current behavior for loose or partially invalid records.

Suggested struct additions:

```go
type Section struct {
    Using *Repo    `yaml:"using"`
    Type  string   `yaml:"type"`
    Repos []Repo   `yaml:"repo"`
    Topics []Topic `yaml:"topics"`
    Record []Record `yaml:"record"`
    // Existing HasRecord and RecordValid stay computed fields.
}
```

Implementation notes:

- Preserve `HasRecord` and `RecordValid`, because callers use the difference between missing `record` and invalid `record`.
- If typed decoding is adopted, add a normalization pass to compute derived booleans.
- Keep current tolerance for malformed nested entries unless tests intentionally tighten behavior.
- Avoid changing `WalkerEvent` public fields.

Primary call sites:

- `service/ghdata/types.go`
- `service/ghdata/walker.go`
- `service/ghdata/check.go`
- `service/workspace/images/check.go`

Tests:

- Extend `service/ghdata/check_test.go` to cover malformed `record` handling.
- Keep workspace image check tests passing.

Acceptance criteria:

- Manual conversion helpers shrink or disappear.
- Invalid `record` values are still detectable.
- `go test ./service/ghdata ./service/workspace/images` passes.

### Phase 4: Reuse Recursive YAML Listing

Simplify `pkg/fileutil/merge.go` by reusing `ListYAMLFilesRecursive`.

Implementation notes:

- Preserve current merge order if callers rely on it. `os.ReadDir` and `ListYAMLFilesRecursive` both return stable sorted paths, but recursive order should be verified in tests.
- Preserve `setCurrentFile` behavior.
- Continue skipping non-YAML files.
- Prefer visible YAML files only, matching `IsYAMLFileName`.

Tests:

- Add focused tests for `ReadAndMergeYAMLFilesRecursive` if none exist.
- Verify hidden files are skipped consistently with `ListYAMLFilesRecursive`.

Acceptance criteria:

- One recursive YAML listing implementation remains.
- `go test ./pkg/fileutil` passes.

## Deferred Ideas

These packages are not recommended for immediate adoption.

- `github.com/rivo/uniseg`: useful only if UI-facing truncation must preserve grapheme clusters such as emoji sequences. Current `TruncateUTF8` already preserves valid UTF-8 runes.
- `github.com/shopspring/decimal`: useful if finance parsing expands beyond two-decimal currency inputs. Current cent parser is explicit and tested.
- `github.com/araddon/dateparse`: too permissive for finance import timestamps. Keep strict layouts in `xzb/internal/parser`.
- `github.com/tidwall/gjson` / `github.com/tidwall/sjson`: not a good fit for YAML-first data paths.
- `github.com/dlclark/regexp2`: no current need for PCRE semantics.
- `colly`, `chromedp`, `rod`: content fetching already has `go-readability`, `goquery`, and `opencli` fallback.

## Risks

- URL extraction refactors can break inbox flushing because byte offsets are used to remove handled URLs from a line.
- Relaxed URL extraction can introduce false positives in prose. Use strict extraction by default.
- Language matching may change which subtitle is selected for ambiguous platform language keys.
- Typed YAML decoding can hide distinctions that current manual map traversal exposes, especially missing vs malformed nested fields.

## Rollout Plan

1. Implement Phase 1 and run targeted tests plus `go test ./...`.
2. Implement Phase 2 independently and run `go test ./service/wiki` plus `go test ./...`.
3. Implement Phase 3 only after adding tests around malformed `record` behavior.
4. Implement Phase 4 as a small fileutil cleanup with focused tests.
5. Run `go mod tidy` only after implementation changes, then inspect diff carefully.

## Validation Commands

```shell
go test ./pkg/urlutil ./service/wiki ./rss2nl/transcript
go test ./service/ghdata ./service/workspace/images
go test ./pkg/fileutil
go test ./...
```

## Decision Summary

Proceed with dependency reuse first:

1. Centralized URL extraction around `xurls`, `goquery`, and existing `urlutil` normalization.
2. Subtitle language matching using `golang.org/x/text/language`.
3. `ghdata` decoding simplification using direct YAML structs or existing `mapstructure`.
4. YAML recursive file listing consolidation using existing `doublestar` helper.

Do not add new broad dependencies until these phases are complete and measured.
