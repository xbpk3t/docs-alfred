package cmd

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/xbpk3t/docs-alfred/pkg/ai"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

// Content types.
type contentType string

const (
	contentVideo contentType = "video"
	contentAudio contentType = "audio"
	contentText  contentType = "text"
)

type inboxEntry struct {
	url       string
	lineIndex int
}

func newWikiCmd() *cobra.Command {
	var config, wikiRoot string
	var inbox bool

	cmd := &cobra.Command{
		Use:   "wiki [urls...]",
		Short: "Classify and summarize URLs into wiki knowledge base",
		Long: `Classify and summarize URLs into wiki knowledge base.
Use --inbox to process wiki/inbox.md. Pass URLs as positional args to process directly.`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if inbox {
				return runWikiInbox(config, wikiRoot)
			}
			if len(args) == 0 {
				slog.Info("No URLs provided and --inbox not set, doing nothing")

				return nil
			}

			return runWikiURLs(config, args, wikiRoot)
		},
	}

	cmd.Flags().StringVarP(&config, "config", "c", "rss2nl.yml", "Config file path")
	cmd.Flags().StringVar(&wikiRoot, "wiki-root", "", "Wiki root directory (overrides config)")
	cmd.Flags().BoolVar(&inbox, "inbox", false, "Read URLs from wiki/inbox.md, process, and flush")

	return cmd
}

func runWikiURLs(config string, urls []string, wikiRoot string) error {
	if _, err := os.Stat(config); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", config)
	}

	fmt.Fprintf(os.Stderr, "Processing %d URL(s) for wiki...\n", len(urls))

	aiCfg := ai.DefaultConfig()

	for _, url := range urls {
		fmt.Fprintf(os.Stderr, "  URL: %s\n", url)
		result := classifyWikiURL(aiCfg, url)
		if result == "" {
			fmt.Fprintf(os.Stderr, "    ⚠ classification unavailable\n")

			continue
		}
		fmt.Fprintf(os.Stderr, "    → %s\n", result)
	}

	return nil
}

func runWikiInbox(config, wikiRoot string) error {
	if _, err := os.Stat(config); os.IsNotExist(err) {
		return fmt.Errorf("config file not found: %s", config)
	}

	if wikiRoot == "" {
		wikiRoot = "wiki"
	}

	inboxPath := filepath.Join(wikiRoot, "inbox.md")
	if _, err := os.Stat(inboxPath); os.IsNotExist(err) {
		return fmt.Errorf("inbox file not found: %s", inboxPath)
	}

	fmt.Fprintf(os.Stderr, "Processing wiki inbox at %s...\n", inboxPath)

	entries, err := parseInbox(inboxPath)
	if err != nil {
		return fmt.Errorf("parse inbox: %w", err)
	}

	if len(entries) == 0 {
		fmt.Fprintf(os.Stderr, "No URLs found in inbox.\n")

		return nil
	}

	fmt.Fprintf(os.Stderr, "Found %d URL(s) in inbox.\n", len(entries))

	aiCfg := ai.DefaultConfig()
	processed := make(map[int]bool)

	for i, entry := range entries {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s\n", i+1, len(entries), entry.url)

		result := classifyWikiURL(aiCfg, entry.url)
		if result == "" {
			fmt.Fprintf(os.Stderr, "    ⚠ classification unavailable, keeping in inbox\n")

			continue
		}
		fmt.Fprintf(os.Stderr, "    → %s\n", result)
		processed[entry.lineIndex] = true
	}

	if len(processed) > 0 {
		flushInbox(inboxPath, processed)
		fmt.Fprintf(os.Stderr, "✅ Flushed %d processed line(s) from inbox.\n", len(processed))
	}

	return nil
}

func classifyWikiURL(cfg *ai.ClientConfig, url string) string {
	// First, determine content type by URL pattern
	urlLower := strings.ToLower(url)
	ct := detectContentType(urlLower)

	_ = ct // used when writing to wiki

	msg := fmt.Sprintf(`Given this URL, classify it into our wiki knowledge base:
URL: %s

Please provide:
1. Topic path (e.g., "infra/k8s" or "works/ai" or "ms/cloud")
2. Type (repo_eval, deep_dive, or inbox)
3. Content type (video, audio, or text)
4. A brief summary (2-3 sentences)

Format as: TOPIC=TYPE/CONTENT_TYPE
Summary: ...`, url)

	result, err := ai.Chat(cfg, []ai.Message{
		{Role: "user", Content: msg},
	})
	if err != nil {
		return ""
	}

	return strings.TrimSpace(result)
}

func detectContentType(urlLower string) contentType {
	if strings.Contains(urlLower, "bilibili.com") ||
		strings.Contains(urlLower, "youtube.com") ||
		strings.Contains(urlLower, "youtu.be") {
		return contentVideo
	}
	if strings.Contains(urlLower, "xiaoyuzhou") ||
		strings.Contains(urlLower, "podcast") ||
		strings.Contains(urlLower, "libsyn.com") {
		return contentAudio
	}

	return contentText
}

func parseInbox(filePath string) ([]inboxEntry, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	reURL := regexp.MustCompile(`https?://[^\s)]+`)
	seen := make(map[string]bool)

	var entries []inboxEntry
	scanner := bufio.NewScanner(file)
	lineIndex := 0
	for scanner.Scan() {
		line := scanner.Text()

		// Find markdown links: [text](url)
		mdLinkRe := regexp.MustCompile(`\[([^\]]*)\]\(([^)]+)\)`)
		for _, m := range mdLinkRe.FindAllStringSubmatch(line, -1) {
			u := m[2]
			if !seen[u] {
				seen[u] = true
				entries = append(entries, inboxEntry{url: u, lineIndex: lineIndex})
			}
		}

		// Find bare URLs
		for _, u := range reURL.FindAllString(line, -1) {
			if !seen[u] {
				seen[u] = true
				entries = append(entries, inboxEntry{url: u, lineIndex: lineIndex})
			}
		}

		lineIndex++
	}

	return entries, scanner.Err()
}

func flushInbox(filePath string, processedLineIndices map[int]bool) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	var remaining []string
	for i, line := range lines {
		if !processedLineIndices[i] {
			remaining = append(remaining, line)
		}
	}

	var nonEmpty []string
	for _, l := range remaining {
		if strings.TrimSpace(l) != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}

	if len(nonEmpty) == 0 {
		_ = os.WriteFile(filePath, []byte{}, fileutil.FilePermPrivate)
	} else {
		_ = os.WriteFile(filepath.Clean(filePath), []byte(strings.Join(remaining, "\n")), fileutil.FilePermPrivate)
	}
}
