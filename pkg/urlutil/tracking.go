package urlutil

import (
	"net/url"
	"strings"

	"github.com/PuerkitoBio/purell"
)

// trackingParams are well-known cross-site tracking/click identifier query
// parameter names. A URL differing only in these parameters is treated as the
// same resource for dedup purposes.
//
// The list is adapted from Miniflux's urlcleaner
// (https://github.com/miniflux/v2/blob/master/internal/reader/urlcleaner/urlcleaner.go,
// Apache-2.0), which in turn cites AdGuard's general_url.txt, Mozilla's
// query-stripping collection, Neat-URL, and Brave's query_filter as sources.
//
// Usage policy: when in doubt, KEEP a parameter. Removing a parameter that
// identifies content (e.g. ?v=, ?bvid=, ?q=) wrongly merges two distinct
// pages; keeping a stray tracking parameter only causes a rare duplicate.
var trackingParams = map[string]bool{
	// Facebook Click Identifiers
	"fbclid":          true,
	"_openstat":       true,
	"fb_action_ids":   true,
	"fb_action_types": true,
	"fb_ref":          true,
	"fb_source":       true,
	"fb_comment_id":   true,

	// Humble Bundles
	"hmb_campaign": true,
	"hmb_medium":   true,
	"hmb_source":   true,

	// Likely Google as well
	"itm_campaign": true,
	"itm_medium":   true,
	"itm_source":   true,

	// Google Click Identifiers
	"gclid":  true,
	"dclid":  true,
	"gbraid": true,
	"wbraid": true,
	"gclsrc": true,

	// Google Analytics
	"campaign_id":      true,
	"campaign_medium":  true,
	"campaign_name":    true,
	"campaign_source":  true,
	"campaign_term":    true,
	"campaign_content": true,

	// Google
	"srsltid": true,

	// Yandex Click Identifiers
	"yclid":  true,
	"ysclid": true,

	// Twitter Click Identifier
	"twclid": true,

	// Microsoft Click Identifier
	"msclkid": true,

	// Mailchimp Click Identifiers
	"mc_cid": true,
	"mc_eid": true,
	"mc_tc":  true,

	// Wicked Reports click tracking
	"wickedid": true,

	// Hubspot Click Identifiers
	"hsa_cam": true,
	"_hsenc":  true,
	"__hssc":  true,
	"__hstc":  true,
	"__hsfp":  true,
	"_hsmi":   true,

	// Olytics
	"rb_clickid":  true,
	"oly_anon_id": true,
	"oly_enc_id":  true,

	// Vero Click Identifier
	"vero_id":   true,
	"vero_conv": true,

	// Marketo email tracking
	"mkt_tok": true,

	// Adobe email tracking
	"sc_cid": true,

	// Beehiiv
	"_bhlid": true,

	// Branch.io
	"_branch_match_id": true,
	"_branch_referrer": true,

	// Readwise
	"__readwiseLocation": true,
}

// trackingParamsPrefixes match parameter families whose names start with the
// given prefix. utm_* comes from the UTM campaign spec; mtm_* is Matomo's
// campaign tracking prefix.
var trackingParamsPrefixes = []string{
	"utm_",
	"mtm_",
}

func isTrackingParam(param string) bool {
	for _, prefix := range trackingParamsPrefixes {
		if strings.HasPrefix(param, prefix) {
			return true
		}
	}

	return trackingParams[param]
}

// RemoveTrackingParams drops known tracking parameters from q. Non-tracking
// params (content identifiers like v, bvid, q, id, page, …) are preserved.
// The input values map is not modified; a new map is returned.
func RemoveTrackingParams(q url.Values) url.Values {
	cleaned := make(url.Values, len(q))
	for k, vs := range q {
		if isTrackingParam(k) {
			continue
		}
		cleaned[k] = vs
	}

	return cleaned
}

// NormalizeForDedup canonicalizes a URL so that URLs differing only in
// tracking parameters, parameter order, fragment, default ports, scheme/host
// case, or a trailing slash compare equal. Content-bearing query parameters
// are preserved.
//
// It layers tracking-parameter removal on top of Normalize, which already
// handles scheme/host case folding, default-port removal, dot-segment
// removal, fragment removal, duplicate-slash removal, query sorting, and
// trailing-slash trimming.
func NormalizeForDedup(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return strings.TrimRight(rawURL, "/")
	}

	// Drop tracking params, then let Normalize do the rest of the folding
	// (sorting re-encodes the remaining query, removes the fragment, folds
	// host case, strips the default port, and trims the trailing slash).
	u.RawQuery = RemoveTrackingParams(u.Query()).Encode()
	normalized, err := purell.NormalizeURLString(u.String(),
		purell.FlagLowercaseScheme|
			purell.FlagLowercaseHost|
			purell.FlagRemoveDefaultPort|
			purell.FlagRemoveDotSegments|
			purell.FlagRemoveFragment|
			purell.FlagRemoveDuplicateSlashes|
			purell.FlagSortQuery)
	if err != nil {
		return strings.TrimRight(u.String(), "/")
	}

	return strings.TrimRight(normalized, "/")
}