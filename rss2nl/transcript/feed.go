package transcript

import (
	"strings"

	"github.com/mmcdole/gofeed"
)

// EpisodeRefFromFeedItem converts a gofeed item into the transcript pipeline input.
func EpisodeRefFromFeedItem(item *gofeed.Item, feedTitle, feedURL string) EpisodeRef {
	if item == nil {
		return EpisodeRef{FeedTitle: feedTitle, FeedURL: feedURL}
	}
	ref := EpisodeRef{
		Title:       item.Title,
		URL:         item.Link,
		GUID:        item.GUID,
		Description: item.Description,
		Content:     item.Content,
		FeedTitle:   feedTitle,
		FeedURL:     feedURL,
	}
	if len(item.Enclosures) > 0 {
		ref.EnclosureURL = item.Enclosures[0].URL
	}

	for ns, extMap := range item.Extensions {
		nsLower := strings.ToLower(ns)
		if !strings.Contains(nsLower, "podcast") && !strings.Contains(nsLower, "transcript") {
			continue
		}
		for tag, exts := range extMap {
			if !strings.Contains(strings.ToLower(tag), "transcript") {
				continue
			}
			for _, e := range exts {
				link := TranscriptLink{URL: e.Attrs["url"], Type: e.Attrs["type"]}
				if link.URL != "" {
					ref.TranscriptLinks = append(ref.TranscriptLinks, link)
				}
			}
		}
	}

	return ref
}
