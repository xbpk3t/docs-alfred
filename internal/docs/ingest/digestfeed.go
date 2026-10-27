package wikiingest

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/xbpk3t/docs-alfred/pkg/md"
)

// digestPrefix is the R2 object prefix for one digest post.
const digestPrefix = "digest/"

var (
	// digestKeyRe matches digest/<batch>/<folder>/<type>/<topic>/<hash>.md.
	digestKeyRe = regexp.MustCompile(`^digest/([^/]+)/([^/]+)/([^/]+)/([^/]+)/([0-9a-f]{16})\.md$`)
	// batchDateRe extracts the date from a wiki-YYYY-MM-DD batch id; non-wiki
	// batches (unbatched) get no pubDate and are skipped from feeds.
	batchDateRe = regexp.MustCompile(`^wiki-(\d{4}-\d{2}-\d{2})$`)
)

// digestPost identifies one post on R2.
type digestPost struct {
	batch     string // wiki-YYYY-MM-DD
	topicPath string // folder/type/topic
	hash      string // sha256[:8] hex, as in the object key
	content   string // the post's markdown body
}

// BuildFeeds lists every digest post in the bucket and rewrites one RSS 2.0
// feed per folder (the first segment of the topic path, e.g. "AI" for
// AI/LLM-use/...). It is a full rebuild: after a digest run the whole folder
// set is re-derived from digest/<batch>/... so feeds never go stale.
// Personal-scale List is fast enough to do this on every run. Folder-level
// feeds keep the subscription count bounded — topic-level would grow one feed
// per topic — while the topic detail stays in each post's title and summary.
func BuildFeeds(ctx context.Context, r2 mediaStore, publicBaseURL string) error {
	posts, err := listDigestPosts(ctx, r2)
	if err != nil {
		return err
	}

	byFolder := make(map[string][]digestPost)
	for _, p := range posts {
		folder := folderOf(p.topicPath)
		byFolder[folder] = append(byFolder[folder], p)
	}
	folders := make([]string, 0, len(byFolder))
	for f := range byFolder {
		folders = append(folders, f)
	}
	sort.Strings(folders)

	for _, f := range folders {
		feedXML, err := buildFeedXML(f, byFolder[f], publicBaseURL)
		if err != nil {
			// Skip one broken folder instead of failing the whole rebuild; it
			// will be retried next run. R2 PUT is not retried here.
			slog.Warn("digest feed: skip folder feed", "folder", f, "error", err)

			continue
		}
		key := "feeds/" + f + ".xml"
		if _, err := r2.Upload(ctx, key, feedXML); err != nil {
			return fmt.Errorf("upload feed %s: %w", key, err)
		}
		slog.Info("digest feed: wrote folder feed", "key", key, "items", len(byFolder[f]))
	}

	return nil
}

// folderOf returns the top-level folder of a topic path. Feeds are grouped at
// folder level, so posts across all types/topics under one folder share a feed.
func folderOf(topicPath string) string {
	if i := strings.Index(topicPath, "/"); i >= 0 {
		return topicPath[:i]
	}

	return topicPath
}

// BuildOPML enumerates every folder feed, builds an OPML 2.0 document with one
// flat <outline> per folder, writes it to R2 as digest.opml, and returns the
// document body (for tests/inspection).
func BuildOPML(ctx context.Context, r2 mediaStore, publicBaseURL string) (string, error) {
	keys, err := r2.ListKeys(ctx, "feeds/")
	if err != nil {
		return "", fmt.Errorf("list feeds: %w", err)
	}

	var folders []string
	for _, key := range keys {
		if f, ok := strings.CutPrefix(key, "feeds/"); ok {
			folders = append(folders, strings.TrimSuffix(f, ".xml"))
		}
	}
	sort.Strings(folders)

	body := opmlBody{}
	for _, folder := range folders {
		// Feeds are folder-level (feeds/<folder>.xml), so each outline is a
		// leaf pointing at its feed directly — no nesting.
		body.Outlines = append(body.Outlines, &opmlOutline{
			Text:  folder,
			Title: folder,
			Feed:  strings.TrimRight(publicBaseURL, "/") + "/feeds/" + folder + ".xml",
		})
	}

	doc := opmlDoc{
		Version: "2.0",
		Head:    opmlHead{Title: "wiki digest"},
		Body:    body,
	}
	raw, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal opml: %w", err)
	}
	out := xml.Header + string(raw) + "\n"
	if _, err := r2.Upload(ctx, "digest.opml", out); err != nil {
		return "", fmt.Errorf("upload digest.opml: %w", err)
	}

	return out, nil
}

// listDigestPosts lists digest/<batch>/... objects and parses each key back
// into a digestPost. Keys that don't match (unexpected layout) are skipped —
// they can't become feed items and would otherwise break the whole rebuild.
func listDigestPosts(ctx context.Context, r2 mediaStore) ([]digestPost, error) {
	keys, err := r2.ListKeys(ctx, digestPrefix)
	if err != nil {
		return nil, fmt.Errorf("list digest posts: %w", err)
	}

	var posts []digestPost
	for _, key := range keys {
		m := digestKeyRe.FindStringSubmatch(key)
		if m == nil {
			slog.Debug("digest feed: skip non-post key", "key", key)

			continue
		}
		content, err := r2.GetObject(ctx, key)
		if err != nil {
			return nil, fmt.Errorf("read digest post %s: %w", key, err)
		}
		posts = append(posts, digestPost{
			batch:     m[1],
			topicPath: m[2] + "/" + m[3] + "/" + m[4],
			hash:      strings.TrimSuffix(m[5], ".md"),
			content:   string(content),
		})
	}

	return posts, nil
}

// buildFeedXML renders one folder's RSS 2.0 XML. Items are sorted newest batch
// first so subscribers see the latest digest at the top.
func buildFeedXML(folder string, posts []digestPost, publicBaseURL string) (string, error) {
	byBatch := make(map[string][]digestPost)
	for _, p := range posts {
		byBatch[p.batch] = append(byBatch[p.batch], p)
	}
	batches := make([]string, 0, len(byBatch))
	for b := range byBatch {
		batches = append(batches, b)
	}
	sort.Strings(batches)

	f := &rssFeed{Version: "2.0"}
	f.Channel = rssChannel{
		Title:       folder,
		Link:        strings.TrimRight(publicBaseURL, "/") + "/digest/" + folder,
		Description: "wiki digest for " + folder,
	}
	// Batches sort lexically ("unbatched" < "wiki-YYYY-MM-DD", and zero-padded
	// dates sort chronologically), so reverse iteration gives newest batch first
	// with unbatched last. Same-batch posts keep their list order.
	var items []rssItem
	lastBuildDate := ""
	for i := len(batches) - 1; i >= 0; i-- {
		b := batches[i]
		date, ok := batchDate(b)
		if ok && lastBuildDate == "" {
			lastBuildDate = date.Format(rfc822Layout)
		}
		for _, p := range byBatch[b] {
			items = append(items, buildItem(folder, p, publicBaseURL, date))
		}
	}
	f.Channel.Items = items
	f.Channel.LastBuildDate = lastBuildDate

	return marshalRSS(f)
}

// buildItem renders one digest post into an RSS item. Title comes from the
// first "### <title>" line of the post body (buildEntry writes h3, not h1);
// link points at the public direct URL of the post's markdown object; and the
// description is the markdown body rendered to HTML via goldmark. A nil date
// (unbatched batch) leaves pubDate blank.
func buildItem(folder string, p digestPost, publicBaseURL string, date *time.Time) rssItem {
	item := rssItem{
		Title:   entryTitle(p.content),
		Link:    digestPublicURL(publicBaseURL, p),
		GUID:    digestPublicURL(publicBaseURL, p),
		PubDate: "",
	}
	if date != nil {
		item.PubDate = date.Format(rfc822Layout)
	}
	if desc, err := md.ToHTML(p.content); err == nil {
		item.Description = desc
	} else {
		slog.Warn("digest feed: markdown to html failed, item without description", "folder", folder, "error", err)
	}

	return item
}

// entryTitle extracts the post title via the shared markdown heading extractor
// (first heading wins). It falls back to a placeholder when there is no heading
// (shouldn't happen for buildEntry output, which always writes an h3).
func entryTitle(mdContent string) string {
	if t := md.ExtractTitleFromMarkdown(mdContent); t != "" {
		return t
	}

	return "(untitled)"
}

// digestPublicURL builds the public link for one post object:
// <publicBaseURL>/digest/<batch>/<topicPath>/<hash>.md
func digestPublicURL(publicBaseURL string, p digestPost) string {
	return fmt.Sprintf("%s/digest/%s/%s/%s.md",
		strings.TrimRight(publicBaseURL, "/"), p.batch, p.topicPath, p.hash)
}

// batchDate returns the time for a batch id ("wiki-YYYY-MM-DD") and ok=false
// for ids that carry no date.
func batchDate(batch string) (*time.Time, bool) {
	m := batchDateRe.FindStringSubmatch(batch)
	if m == nil {
		return nil, false
	}
	t, err := time.Parse(time.DateOnly, m[1])
	if err != nil {
		return nil, false
	}

	return &t, true
}

const rfc822Layout = time.RFC1123Z

// rssFeed mirrors the RSS 2.0 document shape we emit. The xml tags are the
// format contract for consumers (Folo), so their names are stable.
type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	LastBuildDate string    `xml:"lastBuildDate,omitempty"`
	Items         []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate,omitempty"`
	Description string `xml:"description"`
}

func marshalRSS(f *rssFeed) (string, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	if err := xml.NewEncoder(&buf).Encode(f); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// opmlDoc mirrors an OPML 2.0 document. Folo imports the whole document, so the
// outline tree must sit under <body>, not be pasted bare.
type opmlDoc struct {
	XMLName xml.Name `xml:"opml"`
	Version string   `xml:"version,attr"`
	Head    opmlHead `xml:"head"`
	Body    opmlBody `xml:"body"`
}

type opmlHead struct {
	Title string `xml:"title"`
}

// opmlBody is the <body> outline set.
type opmlBody struct {
	Outlines []*opmlOutline `xml:"outline,omitempty"`
}

// opmlOutline is one OPML outline node. OPML 2.0 carries both text and title on
// every <outline>; readers may use either for display, so they are set equal.
// Every outline is a feed leaf; there is no nesting (feeds are folder-level).
type opmlOutline struct {
	Text  string `xml:"text,attr,omitempty"`
	Title string `xml:"title,attr,omitempty"`
	Feed  string `xml:"xmlUrl,attr,omitempty"`
}
