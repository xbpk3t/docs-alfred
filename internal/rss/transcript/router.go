package transcript

import (
	"context"
	"errors"
	"strings"
)

// Router dispatches transcript fetching to the appropriate provider
// based on episode metadata. It implements the Provider interface.
//
// Routing logic:
//  1. Xiaoyuzhou episode (URL/GUID contains xiaoyuzhoufm.com) → XiaoyuzhouProvider
//  2. RSS item has podcast:transcript links → RssTranscriptProvider
//  3. Neither → returns "no transcript source" error
type Router struct {
	Xiaoyuzhou    Provider
	RssTranscript Provider
}

func (r *Router) Name() string {
	return "router"
}

func (r *Router) Fetch(ctx context.Context, ep *EpisodeRef) (*TranscriptResult, error) {
	if isXiaoyuzhouEpisode(ep) {
		// Xiaoyuzhou may be disabled when its credentials are stale; that must
		// not take down RSS-transcript episodes of the same run.
		if r.Xiaoyuzhou == nil {
			return nil, errors.New("xiaoyuzhou transcript unavailable: provider disabled")
		}

		return r.Xiaoyuzhou.Fetch(ctx, ep)
	}

	if len(ep.TranscriptLinks) > 0 {
		return r.RssTranscript.Fetch(ctx, ep)
	}

	return nil, errors.New("no transcript source available for this episode")
}

func isXiaoyuzhouEpisode(ep *EpisodeRef) bool {
	return strings.Contains(ep.GUID, "xiaoyuzhoufm.com") ||
		strings.Contains(ep.URL, "xiaoyuzhoufm.com")
}
