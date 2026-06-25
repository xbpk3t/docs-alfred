package wikiingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	wikisvc "github.com/xbpk3t/docs-alfred/internal/docs/wiki"
)

// DigestLocalInput contains inputs for local directory digest processing.
type DigestLocalInput struct {
	Config  *Config
	deps    *dependencies
	FromDir string
}

// RunDigestLocal processes transcript files from a local directory into the wiki.
// Expected directory structure:
//
//	<fromDir>/
//	  BVxxx_title/
//	    bv.txt          -- BV ID
//	    title.txt       -- video title
//	    transcript.md   -- full subtitle text
func RunDigestLocal(ctx context.Context, input DigestLocalInput) (*Result, error) {
	slog.Info("wiki digest-local started", "fromDir", input.FromDir)
	if input.Config == nil {
		return nil, errors.New("wiki config is required")
	}
	wikiRoot := resolveWikiRoot(input.Config)
	if err := requireDir(wikiRoot, "wiki root"); err != nil {
		return nil, err
	}
	if err := requireDir(input.FromDir, "source dir"); err != nil {
		return nil, err
	}

	deps := resolveDependencies(input.Config, input.deps)
	result := &Result{Name: "wiki digest-local", WikiRoot: wikiRoot}

	entries, err := os.ReadDir(input.FromDir)
	if err != nil {
		return nil, fmt.Errorf("read source dir: %w", err)
	}
	if len(entries) == 0 {
		slog.Info("wiki digest-local: no subdirectories found")

		return result, nil
	}

	slog.Info("wiki digest-local: processing subdirectories", "count", len(entries))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirPath := filepath.Join(input.FromDir, entry.Name())
		urlResult := processLocalDir(ctx, deps, wikiRoot, dirPath)
		result.URLResults = append(result.URLResults, urlResult)
	}

	return result, nil
}

// localInputs holds the parsed contents of a local transcript directory.
type localInputs struct {
	bv      string
	title   string
	content string
}

// readLocalInputs reads and validates bv.txt, title.txt, and transcript.md from dirPath.
func readLocalInputs(dirPath string) (*localInputs, error) {
	dirName := filepath.Base(dirPath)

	bvBytes, err := os.ReadFile(filepath.Join(dirPath, "bv.txt"))
	if err != nil {
		return nil, fmt.Errorf("%s: read bv.txt: %w", dirName, err)
	}
	bv := strings.TrimSpace(string(bvBytes))
	if bv == "" {
		return nil, fmt.Errorf("%s: empty bv.txt", dirName)
	}

	titleBytes, err := os.ReadFile(filepath.Join(dirPath, "title.txt"))
	if err != nil {
		return nil, fmt.Errorf("%s: read title.txt: %w", dirName, err)
	}
	title := strings.TrimSpace(string(titleBytes))
	if title == "" {
		title = bv
	}

	contentBytes, err := os.ReadFile(filepath.Join(dirPath, "transcript.md"))
	if err != nil {
		return nil, fmt.Errorf("%s: read transcript.md: %w", dirName, err)
	}
	content := strings.TrimSpace(string(contentBytes))
	if len([]rune(content)) < 200 {
		return nil, fmt.Errorf("%s: transcript.md too short (%d chars)", dirName, len([]rune(content)))
	}

	return &localInputs{bv: bv, title: title, content: content}, nil
}

// processLocalDir handles one transcript directory: reads, classifies, copies transcript, writes summary.
func processLocalDir(ctx context.Context, deps *dependencies, wikiRoot, dirPath string) URLResult {
	dirName := filepath.Base(dirPath)

	inputs, err := readLocalInputs(dirPath)
	if err != nil {
		return skipResult(dirName, err.Error())
	}

	url := fmt.Sprintf("https://www.bilibili.com/video/%s/", inputs.bv)
	slog.Info("Processing local transcript", "bv", inputs.bv, "title", inputs.title)

	contentType := wikisvc.DetectContentType(url)

	// Pre-classification quality gate for video content.
	if contentType == wikisvc.ContentVideo && len([]rune(inputs.content)) < 600 {
		item := &wikisvc.ClassifyItem{
			URL: url, Title: inputs.title,
			ContentType: contentType,
			Summary:     &wikisvc.StructuredSummary{Overview: inputs.content},
		}
		urlResult := writePendingURL(deps, wikiRoot, &pendingURLWrite{
			URL: url, Kind: pendingExtractFailure,
			Item: item, FailureType: wikisvc.FailureExtract,
			ExtraInfo: "video content too short",
		}, false)
		urlResult.Handled = true

		return urlResult
	}

	classResult := deps.classifier.ClassifyURL(ctx, url, inputs.title, inputs.content)
	if classResult == nil {
		item := &wikisvc.ClassifyItem{
			URL: url, Title: inputs.title,
			ContentType: contentType,
			Summary:     &wikisvc.StructuredSummary{Overview: inputs.content},
		}
		if strings.TrimSpace(inputs.content) == "" {
			return writePendingURL(deps, wikiRoot, &pendingURLWrite{
				URL: url, Kind: pendingExtractFailure,
				Item: item, FailureType: wikisvc.FailureExtract,
				ExtraInfo: "empty content",
			}, false)
		}

		return URLResult{URL: url, Status: StatusUnhandledError, Error: "classification failed: AI error"}
	}

	item := &wikisvc.ClassifyItem{
		URL:               url,
		Title:             inputs.title,
		ContentType:       classResult.ContentType,
		TopicPath:         classResult.TopicPath,
		Type:              classResult.WikiType,
		Summary:           classResult.Summary,
		MetadataBlock:     classResult.MetadataBlock,
		NeedsManualReview: classResult.NeedsManualReview,
	}

	// Append transcript reference to metadata block.
	transcriptRelPath := fmt.Sprintf("transcript/transcript-%s.md", inputs.bv)
	if item.MetadataBlock != "" {
		item.MetadataBlock += "\n"
	}
	item.MetadataBlock += "Transcript: " + transcriptRelPath

	// Always copy transcript to wiki dir (even if classification fails).
	if topicPath := classResult.TopicPath; topicPath != "" {
		_ = copyTranscriptToWiki(dirPath, wikiRoot, topicPath, inputs.bv)
	}

	if shouldWriteClassifyFailure(classResult) {
		extraInfo := classifyFailureInfo(classResult)

		return writePendingURL(deps, wikiRoot, &pendingURLWrite{
			URL: url, Kind: pendingClassifyFailure,
			Item: item, FailureType: wikisvc.FailureClassify,
			ExtraInfo: extraInfo,
		}, false)
	}

	return writePendingURL(deps, wikiRoot, &pendingURLWrite{
		URL:  url,
		Kind: pendingSummary,
		Item: item,
	}, false)
}

// copyTranscriptToWiki copies transcript.md from sourceDir to wiki/<topicPath>/transcript/transcript-<BV>.md.
func copyTranscriptToWiki(srcDir, wikiRoot, topicPath, bv string) error {
	src := filepath.Join(srcDir, "transcript.md")
	dstDir := filepath.Clean(filepath.Join(wikiRoot, topicPath, "transcript"))
	if !strings.HasPrefix(dstDir, filepath.Clean(wikiRoot)) {
		return fmt.Errorf("destination path escapes wiki root: %s", dstDir)
	}
	dst := filepath.Clean(filepath.Join(dstDir, fmt.Sprintf("transcript-%s.md", filepath.Base(bv))))
	if !strings.HasPrefix(dst, dstDir) {
		return fmt.Errorf("destination file escapes transcript dir: %s", dst)
	}

	if err := os.MkdirAll(dstDir, 0o750); err != nil {
		return fmt.Errorf("create transcript dir: %w", err)
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("read source transcript: %w", err)
	}

	if err := os.WriteFile(dst, data, 0o600); err != nil { //nolint:gosec // dst validated above via HasPrefix check
		return fmt.Errorf("write transcript to wiki: %w", err)
	}

	slog.Info("Transcript copied to wiki", "bv", bv, "dst", dst)

	return nil
}

// skipResult returns a handled failure result for a directory that can't be processed.
func skipResult(dirName, reason string) URLResult {
	slog.Warn("wiki digest-local: skip directory", "dir", dirName, "reason", reason)

	return URLResult{
		URL:     dirName,
		Status:  StatusFailureWritten,
		Handled: true,
	}
}
