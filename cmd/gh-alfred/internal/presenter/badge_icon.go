package presenter

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/xbpk3t/docs-alfred/internal/gh/content"
	"github.com/xbpk3t/docs-alfred/internal/gh/index"
	"github.com/xbpk3t/docs-alfred/pkg/fileutil"
)

var repoIconCacheDir = fileutil.CachePath("gh-alfred/icons")

const badgeCheckPath = "M29.28 6.362a2.502 2.502 0 0 0-3.458.736L14.936 23.877l-5.029-4.65" +
	"a2.5 2.5 0 1 0-3.394 3.671l7.209 6.666c.48.445 1.09.665 1.696.665" +
	"c.673 0 1.534-.282 2.099-1.139c.332-.506 12.5-19.27 12.5-19.27" +
	"a2.5 2.5 0 0 0-.737-3.458z"

type badgeState struct {
	HasDoc bool
	HasNix bool
	Score  int
}

func repoIconPath(repo *content.Repo) string {
	if repo == nil {
		return IconGh
	}
	path, ok := ensureBadgeIcon(repoBadgeState(repo))
	if !ok {
		return IconGh
	}

	return path
}

func repoBadgeState(repo *content.Repo) badgeState {
	if repo == nil {
		return badgeState{}
	}

	return badgeState{
		HasDoc: strings.TrimSpace(repo.Doc) != "",
		HasNix: ghindex.HasNix(repo),
		Score:  clampScore(repo.Score),
	}
}

func clampScore(score int) int {
	switch {
	case score < 0:
		return 0
	case score > 5:
		return 5
	default:
		return score
	}
}

func ensureBadgeIcon(state badgeState) (string, bool) {
	path := badgeIconPath(state)
	if _, err := os.Stat(path); err == nil {
		return path, true
	}

	if err := fileutil.AtomicWriteFile(path, []byte(renderBadgeIconSVG(state)), fileutil.FilePermPrivate); err != nil {
		return "", false
	}

	return path, true
}

func badgeIconPath(state badgeState) string {
	return filepath.Join(repoIconCacheDir, badgeIconName(state))
}

func badgeIconName(state badgeState) string {
	return fmt.Sprintf("gh-d%d-n%d-s%d.svg", boolInt(state.HasDoc), boolInt(state.HasNix), clampScore(state.Score))
}

func boolInt(v bool) int {
	if v {
		return 1
	}

	return 0
}

func renderBadgeIconSVG(state badgeState) string {
	return fmt.Sprintf(`<svg width="800px" height="800px" viewBox="0 0 36 36" xmlns="http://www.w3.org/2000/svg"
  aria-hidden="true" role="img" preserveAspectRatio="xMidYMid meet">
  <path fill="#77B255" d="M36 32a4 4 0 0 1-4 4H4a4 4 0 0 1-4-4V4a4 4 0 0 1 4-4h28a4 4 0 0 1 4 4v28z"/>
  <path fill="#FFF" d="%s"/>
  <rect x="2" y="24.3" width="32" height="10.2" rx="2.8" fill="#111827" opacity="0.82"/>
  %s
  %s
  %s
</svg>`, badgeCheckPath, docBadgeSVG(state.HasDoc), nixBadgeSVG(state.HasNix), scoreBadgeSVG(state.Score))
}

func docBadgeSVG(active bool) string {
	return textBadgeSVG(4, 7.5, badgeFill(active, "#F97316"), badgeText(active, "#FFFFFF"), "D")
}

func nixBadgeSVG(active bool) string {
	return textBadgeSVG(13.5, 17.0, badgeFill(active, "#DC2626"), badgeText(active, "#FFFFFF"), "N")
}

func scoreBadgeSVG(score int) string {
	return textBadgeSVG(23, 27.5, "#FABC05", "#111827", strconv.Itoa(clampScore(score)))
}

func textBadgeSVG(x, textX float64, fill, textFill, label string) string {
	return fmt.Sprintf(`<g>
    <rect x="%.1f" y="25.6" width="9" height="7.8" rx="2" fill="%s"/>
    <text x="%.1f" y="31.8" text-anchor="middle" font-family="Arial, Helvetica, sans-serif" font-size="6.3"
      font-weight="700" fill="%s">%s</text>
  </g>`, x, fill, textX, textFill, label)
}

func badgeFill(active bool, color string) string {
	if active {
		return color
	}

	return "#4B5563"
}

func badgeText(active bool, color string) string {
	if active {
		return color
	}

	return "#D1D5DB"
}
