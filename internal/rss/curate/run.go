package curate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	rss "github.com/xbpk3t/docs-alfred/internal/rss/feed"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// RunOptions carries everything a curate invocation needs.
type RunOptions struct {
	Now        time.Time
	RSS2NLPath string
	ConfigPath string
	OutDir     string
	CachePath  string
	SeedPaths  []string
}

// RunResult is the outcome of a curate run.
type RunResult struct {
	Keeps    []keepEntry
	Drops    []dropEntry
	Failed   []string
	FreqPass []freqResult
	Summary  Summary
}

// mediaType marks media types whose feeds set rss.Feeds.IsMedia.
var mediaType = map[string]bool{
	"video": true, "youtube": true, "podcast": true, "vlog": true, "audio": true,
}

// Run drives the whole pipeline. It only writes under OutDir (preview) plus an
// optional cache file; it never appends to rss2nl.yml, sends, or commits.
func Run(ctx context.Context, opts *RunOptions) (*RunResult, error) {
	cfg, err := LoadConfig(opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	rssCfg, err := rss.NewConfig(opts.RSS2NLPath)
	if err != nil {
		return nil, fmt.Errorf("load rss2nl %s: %w", opts.RSS2NLPath, err)
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	outDir := opts.OutDir
	if outDir == "" {
		outDir = filepath.Join(os.TempDir(), fmt.Sprintf("rss-curate-%s", now.Format("20060102-150405")))
	}
	if err = fileutil.EnsureDir(outDir); err != nil {
		return nil, fmt.Errorf("create out dir: %w", err)
	}

	// ---- Step 1: candidates ----
	seeds := &SeedFiles{Sources: opts.SeedPaths}
	if len(opts.SeedPaths) > 0 {
		if seeds, err = LoadSeeds(opts.SeedPaths); err != nil {
			return nil, err
		}
	}
	candidates, err := detectCandidates(ctx, cfg, rssCfg, seeds)
	if err != nil {
		return nil, err
	}

	// ---- Step 2: fetch + frequency gate, cache-friendly so AI retries skip it ----
	results, err := step2OrReload(ctx, rssCfg, cfg, candidates, now, opts.CachePath)
	if err != nil {
		return nil, err
	}
	pass := passResults(results)

	// ---- Step 3/4: classify feeds, then per-type curator ----
	classByFeed, err := classifyPassed(ctx, cfg, rssCfg, pass)
	if err != nil {
		return nil, err
	}
	typeFailed, verdicts := curateBatch(ctx, cfg, pass, classByFeed)

	// ---- Step 5: assemble and write the preview artifacts ----
	pv, err := assemblePreview(outDir, results, classByFeed, verdicts, typeFailed, seeds.Sources, cfg, len(candidates), now)
	if err != nil {
		return nil, err
	}

	return &RunResult{
		Summary:  pv.Summary,
		Keeps:    pv.Keeps,
		Drops:    pv.Drops,
		Failed:   pv.Failed,
		FreqPass: pass,
	}, nil
}

// ---- Step 1-5 helpers (split out of Run to keep it a short orchestrator) ----

// finalResults is the assembled, persisted outcome of a curate run.
type finalResults struct {
	Keeps   []keepEntry
	Drops   []dropEntry
	Failed  []string
	Summary Summary
}

// detectCandidates resolves the seed list: deterministic URL extraction first,
// the MAF resolver only for prose-only lists, then the feed gate finalizer.
func detectCandidates(ctx context.Context, cfg *Config, rssCfg *rss.Config, seeds *SeedFiles) ([]Candidate, error) {
	cands := extractDeterministic(seeds.Content)
	if len(cands) == 0 {
		var err error
		if cands, err = step1Resolve(ctx, cfg, resolvePrompt{
			Seed:       seeds.Content,
			Aliases:    aliasesLines(cfg.KnownAliases),
			Existing:   existingTypes(rssCfg),
			XZTemplate: "RSSHub 小宇宙模板：https://rsshub.app/xiaoyuzhou/podcast/<id>",
		}); err != nil {
			return nil, err
		}
	}
	for i := range cands {
		finalizeCandidate(&cands[i], cfg.KnownAliases)
	}

	return cands, nil
}

// passResults returns only the Step-2 rows that cleared the frequency gate.
func passResults(results []freqResult) []freqResult {
	pass := make([]freqResult, 0, len(results))
	for _, r := range results {
		if r.Status == statusPass {
			pass = append(pass, r)
		}
	}

	return pass
}

// classifyPassed runs the Step-3 classifier over the frequency-passed pool,
// mapping each feed to one existing rss2nl type.
func classifyPassed(ctx context.Context, cfg *Config, rssCfg *rss.Config, pool []freqResult) (map[string]string, error) {
	classByFeed := map[string]string{}
	if len(pool) == 0 {
		return classByFeed, nil
	}
	items := make([]classInputItem, 0, len(pool))
	for _, r := range pool {
		items = append(items, classInputItem{Name: r.Candidate.Name, Medium: r.Candidate.Medium, Feed: r.Candidate.Feed, Titles: r.Titles})
	}
	if limit := cfg.MaxCandidates; limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	assigns, err := step3Classify(ctx, cfg, rTypes(rssCfg), items)
	if err != nil {
		return nil, fmt.Errorf("step3 classify: %w", err)
	}
	for _, a := range assigns {
		if a.Feed != "" {
			classByFeed[normalizer(a.Feed)] = a.Type
		}
	}

	return classByFeed, nil
}

// curateBatch fans out one per-type curator call, isolating whole-type failures
// from the per-feed verdicts it returns.
func curateBatch(ctx context.Context, cfg *Config, pool []freqResult, classByFeed map[string]string) (map[string]string, map[string]verdict) {
	typeFailed := map[string]string{}
	verdictByName := map[string]verdict{}
	if len(pool) == 0 {
		return typeFailed, verdictByName
	}
	for _, g := range step4Curate(ctx, cfg, groupCurate(pool, classByFeed)) {
		if g.Failed {
			typeFailed[g.Type] = g.Err
			continue
		}
		for _, v := range g.Verdicts {
			verdictByName[canonicalName(v.Name)] = v
		}
	}

	return typeFailed, verdictByName
}

// assemblePreview converts the Step 2-4 outputs into keeps/drops and writes the
// YAML/keep/ drops/summary previews under outDir.
func assemblePreview(outDir string, results []freqResult, classByFeed map[string]string, verdicts map[string]verdict, typeFailed map[string]string, sources []string, cfg *Config, resolved int, now time.Time) (finalResults, error) {
	var pv finalResults
	keeps, drops, unassigned := accumulateResult(passResults(results), classByFeed, verdicts, typeFailed)
	drops = append(mechanicalDrops(results), drops...)

	if err := writeYAMLPreview(outDir, keeps, now); err != nil {
		return pv, err
	}
	if err := writeKeepPreview(outDir, keeps, now); err != nil {
		return pv, err
	}
	if err := writeDropsPreview(outDir, drops, unassigned, typeFailed, now); err != nil {
		return pv, err
	}

	pv.Summary = Summary{
		GeneratedAt: now.Format(time.RFC3339),
		SeedFiles:   sources,
		Resolved:    resolved,
		FreqPass:    len(passResults(results)),
		FreqFail:    len(results) - len(passResults(results)),
		Kept:        len(keeps),
		Dropped:     len(drops),
		Model:       cfg.Model,
		Posts90DMin: cfg.Posts90DMin,
		LastMaxDays: cfg.LastMaxDays,
	}
	if err := writeSummaryPreview(outDir, &pv.Summary); err != nil {
		return pv, err
	}
	pv.Keeps, pv.Drops, pv.Failed = keeps, drops, failuresList(unassigned, typeFailed)

	return pv, nil
}

// writeYAMLPreview renders and writes the keep preview YAML.
func writeYAMLPreview(outDir string, keeps []keepEntry, now time.Time) error {
	yamlBody, err := renderKeepYAML(groupKeepsByType(keeps), now)
	if err != nil {
		return err
	}

	return fileutil.AtomicWriteFile(filepath.Join(outDir, "bestblogs-keep-preview.yml"), yamlBody, fileutil.FilePermPrivate)
}

// writeKeepPreview writes the human-readable keep markdown.
func writeKeepPreview(outDir string, keeps []keepEntry, now time.Time) error {
	return fileutil.AtomicWriteFile(filepath.Join(outDir, "bestblogs-keep.md"), []byte(renderKeepsMarkdown(keeps, now)), fileutil.FilePermPrivate)
}

// writeDropsPreview writes the per-reason drops report, including un-scored rows.
func writeDropsPreview(outDir string, drops []dropEntry, unassigned []string, typeFailed map[string]string, now time.Time) error {
	failed := map[string]string{}
	for _, f := range unassigned {
		failed[f] = "unrated"
	}
	for t, msg := range typeFailed {
		failed["type:"+t] = msg
	}

	return fileutil.AtomicWriteFile(filepath.Join(outDir, "bestblogs-drops.md"), []byte(renderDropsMarkdown(drops, failed, now)), fileutil.FilePermPrivate)
}

// writeSummaryPreview writes the machine-readable run summary.
func writeSummaryPreview(outDir string, summary *Summary) error {
	payload, err := fileutil.MarshalJSON(summary)
	if err != nil {
		return err
	}

	return fileutil.AtomicWriteFile(filepath.Join(outDir, "summary.json"), payload, fileutil.FilePermPrivate)
}

// canonicalName keys the curator verdicts by the same name form used elsewhere.
func canonicalName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// ---- Step 5 assembly helpers ----

// accumulateResult turns the Step 2-4 outputs into keeps/drops/unassigned.
func accumulateResult(
	pass []freqResult,
	classByFeed map[string]string,
	verdictByName map[string]verdict,
	typeFailed map[string]string,
) (keeps []keepEntry, drops []dropEntry, unassigned []string) {
	for _, r := range pass {
		c := r.Candidate
		feed := normalizer(c.Feed)
		typ := classByFeed[feed]
		if typ == "" {
			unassigned = append(unassigned, c.Name+" (no type)")
			continue
		}
		if _, bad := typeFailed[typ]; bad {
			unassigned = append(unassigned, c.Name+" (type failed)")
			continue
		}
		v, ok := verdictByName[strings.ToLower(strings.TrimSpace(c.Name))]
		if !ok {
			unassigned = append(unassigned, c.Name+" (unrated)")
			continue
		}
		if v.Decision == "drop" {
			drops = append(drops, dropEntry{Name: c.Name, Feed: c.Feed, Status: v.ReasonCode, Reason: v.ReasonText})
			continue
		}
		keeps = append(keeps, keepEntry{
			Type:    typ,
			Feed:    c.Feed,
			URL:     c.URL,
			Des:     c.Note,
			IsMedia: mediaType[strings.ToLower(c.Medium)],
		})
	}

	return keeps, drops, unassigned
}

// mechanicalDrops turns every Step-2 reject (unreachable, freq_low, no_rss,
// dedupe, invalid_url) into a drop row so drops.md explains all rejects.
func mechanicalDrops(results []freqResult) []dropEntry {
	var out []dropEntry
	for _, r := range results {
		if r.Status == statusPass {
			continue
		}
		out = append(out, dropEntry{Name: r.Candidate.Name, Feed: r.Candidate.Feed, Status: r.Status, Reason: r.Reason})
	}

	return out
}

// step2OrReload fetches + filters, but reuses a cache whose row count matches
// (deterministic extraction yields the same candidates each run, so 1:1 reuse
// is safe). This makes downstream AI-step retries cheap — no second fetch.
func step2OrReload(
	ctx context.Context,
	feedCfg *rss.Config,
	cfg *Config,
	cands []Candidate,
	now time.Time,
	cachePath string,
) ([]freqResult, error) {
	if cachePath != "" {
		if c, err := loadCache(cachePath); err == nil && len(c.Rows) == len(cands) {
			return c.Rows, nil
		}
	}
	results, err := step2Fetch(ctx, feedCfg, cfg, cands, now)
	if err != nil {
		return nil, err
	}
	if cachePath != "" {
		_ = (&fetchCache{Rows: results}).save(cachePath)
	}

	return results, nil
}

// failuresList flattens unassigned + type failures into one list for summary.
func failuresList(unassigned []string, typeFailed map[string]string) []string {
	out := make([]string, len(unassigned), len(unassigned)+len(typeFailed))
	copy(out, unassigned)
	for t, msg := range typeFailed {
		out = append(out, fmt.Sprintf("type %s: %s", t, msg))
	}

	return out
}

// ---------------------------------------------------------------------------
// Prompt-context helpers (pure, deterministic — no AI).
// ---------------------------------------------------------------------------

// aliasesLines renders known aggregate aliases into name -> feed lines.
func aliasesLines(known map[string]string) string {
	if len(known) == 0 {
		return ""
	}
	var b strings.Builder
	for name, feed := range known {
		fmt.Fprintf(&b, "- %s -> %s\n", name, feed)
	}

	return b.String()
}

// rTypes returns the ordered list of existing rss2nl.yml types.
func rTypes(cfg *rss.Config) []string {
	out := make([]string, 0, len(cfg.RSS))
	for _, d := range cfg.RSS {
		if d.Type != "" {
			out = append(out, d.Type)
		}
	}

	return out
}

// existingTypes maps each type to its feed URLs for the resolver prompt.
func existingTypes(cfg *rss.Config) map[string][]string {
	out := make(map[string][]string, len(cfg.RSS))
	for _, d := range cfg.RSS {
		var feeds []string
		for _, f := range d.Feeds {
			if f.Feed != "" {
				feeds = append(feeds, f.Feed)
			}
		}
		out[d.Type] = feeds
	}

	return out
}

// groupCurate splits the pass pool per classified type for the fanout.
func groupCurate(pass []freqResult, classByFeed map[string]string) map[string][]curateItem {
	groups := make(map[string][]curateItem)
	for _, r := range pass {
		typ := classByFeed[normalizer(r.Candidate.Feed)]
		groups[typ] = append(groups[typ], curateItem{
			Name:   r.Candidate.Name,
			Medium: r.Candidate.Medium,
			Feed:   r.Candidate.Feed,
			Note:   r.Candidate.Note,
			Titles: r.Titles,
		})
	}

	return groups
}
