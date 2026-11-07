package wikiingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fakeYTDLPScript = `#!/bin/sh
case " $* " in
  *" --dump-json "*)
    printf '%s\n' '{"title":"Test Video","subtitles":{"en":[{"ext":"vtt"}]},"automatic_captions":{}}'
    ;;
  *" --write-subs "*|*" --write-auto-subs "*)
    dir=""
    prev=""
    for a in "$@"; do
      if [ "$prev" = "-o" ]; then dir=$(dirname "$a"); fi
      prev="$a"
    done
    printf 'WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nHello from the subtitle.\n' > "$dir/vid.en.vtt"
    ;;
esac
`

// TestNewWikiFetcherUsesConfiguredVideoDriver proves wiki.driver switches the
// fetcher to the yt-dlp-backed drivers without touching the opencli default.
func TestNewWikiFetcherUsesConfiguredVideoDriver(t *testing.T) {
	for _, driverName := range []string{"ytdlp", "hybrid"} {
		t.Run(driverName, func(t *testing.T) {
			binDir := t.TempDir()
			require.NoError(t, os.WriteFile(
				filepath.Join(binDir, "yt-dlp"), []byte(fakeYTDLPScript), 0o700,
			))
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			cfg := &Config{}
			cfg.Wiki.Driver = driverName
			cfg.Wiki.MaxContentSize = 20000
			cfg.Wiki.Media.Enabled = true

			fetcher := newWikiFetcher(cfg)
			result := fetcher.FetchContent(
				context.Background(), "https://www.youtube.com/watch?v=abc", "",
			)

			require.NotNil(t, result)
			require.Empty(t, result.Error)
			assert.True(t, result.HasTranscript)
			assert.Contains(t, result.Body, "## 字幕内容")
		})
	}
}
