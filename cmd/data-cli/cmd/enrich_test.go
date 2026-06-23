package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
	data "github.com/xbpk3t/docs-alfred/internal/gh/domrules"
	"github.com/xbpk3t/docs-alfred/internal/gh/enrich"
)

// ---------------------------------------------------------------------------
// parseEnrichResourceArg
// ---------------------------------------------------------------------------

func TestParseEnrichResourceArgValid(t *testing.T) {
	tests := []struct {
		input string
		want  enrich.ResourceType
	}{
		{"movie", enrich.ResourceMovie},
		{"tv", enrich.ResourceTV},
		{"book", enrich.ResourceBook},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseEnrichResourceArg(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseEnrichResourceArgInvalid(t *testing.T) {
	tests := []string{"music", "podcast", "", "Movie"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := parseEnrichResourceArg(input)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unsupported enrichment resource")
		})
	}
}

// ---------------------------------------------------------------------------
// envVarForResource
// ---------------------------------------------------------------------------

func TestEnvVarForResourceMovieAndTVUseTMDB(t *testing.T) {
	for _, rt := range []enrich.ResourceType{enrich.ResourceMovie, enrich.ResourceTV} {
		t.Run(string(rt), func(t *testing.T) {
			envVar, label := envVarForResource(rt)
			require.Equal(t, "TMDB_API_KEY", envVar)
			require.Equal(t, "TMDB", label)
		})
	}
}

func TestEnvVarForResourceBookUsesGoogle(t *testing.T) {
	envVar, label := envVarForResource(enrich.ResourceBook)
	require.Equal(t, "GOOGLE_CLOUD_API_KEY", envVar)
	require.Equal(t, "Google Books", label)
}

func TestEnvVarForResourceUnknownReturnsEmpty(t *testing.T) {
	envVar, label := envVarForResource(enrich.ResourceType("unknown"))
	require.Empty(t, envVar)
	require.Empty(t, label)
}

// ---------------------------------------------------------------------------
// resolveAPIKey
// ---------------------------------------------------------------------------

func TestResolveAPIKeyReturnsKeyWhenSet(t *testing.T) {
	tests := []struct {
		name   string
		rt     enrich.ResourceType
		envVar string
	}{
		{"movie", enrich.ResourceMovie, "TMDB_API_KEY"},
		{"tv", enrich.ResourceTV, "TMDB_API_KEY"},
		{"book", enrich.ResourceBook, "GOOGLE_CLOUD_API_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envVar, "test-key-123")

			key, err := resolveAPIKey(tt.rt)
			require.NoError(t, err)
			require.Equal(t, "test-key-123", key)
		})
	}
}

func TestResolveAPIKeyErrorsWhenUnset(t *testing.T) {
	tests := []struct {
		name   string
		rt     enrich.ResourceType
		envVar string
		label  string
	}{
		{"movie missing TMDB key", enrich.ResourceMovie, "TMDB_API_KEY", "TMDB"},
		{"tv missing TMDB key", enrich.ResourceTV, "TMDB_API_KEY", "TMDB"},
		{"book missing Google key", enrich.ResourceBook, "GOOGLE_CLOUD_API_KEY", "Google Books"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envVar, "")

			_, err := resolveAPIKey(tt.rt)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.envVar)
			require.Contains(t, err.Error(), tt.label)
		})
	}
}

// ---------------------------------------------------------------------------
// parseEnrichFlags
// ---------------------------------------------------------------------------

func TestParseEnrichFlagsDefaultValues(t *testing.T) {
	cmd := newEnrichCmd()
	flags := parseEnrichFlags(cmd)

	require.Empty(t, flags.path)
	require.Empty(t, flags.cache)
	require.False(t, flags.dryRun)
}

func TestParseEnrichFlagsSetValues(t *testing.T) {
	cmd := newEnrichCmd()
	require.NoError(t, cmd.Flags().Set("path", "/tmp/movie.yml"))
	require.NoError(t, cmd.Flags().Set("cache", "/tmp/cache.json"))
	require.NoError(t, cmd.Flags().Set("dry-run", "true"))

	flags := parseEnrichFlags(cmd)

	require.Equal(t, "/tmp/movie.yml", flags.path)
	require.Equal(t, "/tmp/cache.json", flags.cache)
	require.True(t, flags.dryRun)
}

// ---------------------------------------------------------------------------
// newEnrichCmd
// ---------------------------------------------------------------------------

func TestNewEnrichCmdStructure(t *testing.T) {
	cmd := newEnrichCmd()

	require.Equal(t, "enrich <resource>", cmd.Use)
	require.Contains(t, cmd.Short, "Enrich")
	require.True(t, cmd.SilenceUsage)
	require.NotNil(t, cmd.RunE)
}

func TestNewEnrichCmdFlags(t *testing.T) {
	cmd := newEnrichCmd()

	require.NotNil(t, cmd.Flag("path"))
	require.NotNil(t, cmd.Flag("cache"))
	require.NotNil(t, cmd.Flag("dry-run"))
}

func TestNewEnrichCmdRequiresExactlyOneArg(t *testing.T) {
	cmd := newEnrichCmd()

	// No args should error.
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s)")

	// Two args should also error.
	cmd.SetArgs([]string{"movie", "tv"})
	err = cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s)")
}

// ---------------------------------------------------------------------------
// parseDataDomainArg
// ---------------------------------------------------------------------------

func TestParseDataDomainArgValid(t *testing.T) {
	tests := []struct {
		input string
		want  data.DataDomain
	}{
		{"gh", data.DomainGH},
		{"books", data.DomainBooks},
		{"movie", data.DomainMovie},
		{"tv", data.DomainTV},
		{"music", data.DomainMusic},
		{"diary", data.DomainDiary},
		{"goods", data.DomainGoods},
		{"task", data.DomainTask},
		{"ntl", data.DomainNtl},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDataDomainArg(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseDataDomainArgUnknown(t *testing.T) {
	tests := []string{"unknown", "podcast", "", "GH"}

	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			_, err := parseDataDomainArg(input)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown data domain")
		})
	}
}

// ---------------------------------------------------------------------------
// newRenderCmd
// ---------------------------------------------------------------------------

func TestNewRenderCmdFlags(t *testing.T) {
	cmd := newRenderCmd()

	require.Equal(t, "render", cmd.Name())
	require.NotNil(t, cmd.Flag("config"))
	require.NotNil(t, cmd.Flag("extract"))
	require.NotNil(t, cmd.Flag("out"))

	// Check default for --config.
	require.Equal(t, "docs.yml", cmd.Flag("config").DefValue)
}

// ---------------------------------------------------------------------------
// newCheckCmd
// ---------------------------------------------------------------------------

func TestNewCheckCmdFlags(t *testing.T) {
	cmd := newCheckCmd()

	require.Equal(t, "check", cmd.Name())
	require.Equal(t, "check <domain>", cmd.Use)
	require.NotNil(t, cmd.Flag("path"))
	require.NotNil(t, cmd.Flag("max-lines"))
	require.NotNil(t, cmd.Flag("rule-scope"))

	// rule-scope is hidden.
	require.True(t, cmd.Flag("rule-scope").Hidden)
}

func TestNewCheckCmdRequiresExactlyOneArg(t *testing.T) {
	cmd := newCheckCmd()

	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s)")
}

// ---------------------------------------------------------------------------
// newDuplicateCmd
// ---------------------------------------------------------------------------

func TestNewDuplicateCmdFlags(t *testing.T) {
	cmd := newDuplicateCmd()

	require.Equal(t, "duplicate", cmd.Name())
	require.Equal(t, "duplicate <domain>", cmd.Use)
	require.NotNil(t, cmd.Flag("path"))
}

func TestNewDuplicateCmdRequiresExactlyOneArg(t *testing.T) {
	cmd := newDuplicateCmd()

	cmd.SetArgs([]string{})
	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "accepts 1 arg(s)")
}

// ---------------------------------------------------------------------------
// newGhFindCmd
// ---------------------------------------------------------------------------

func TestNewGhFindCmdFlags(t *testing.T) {
	cmd := newGhFindCmd()

	require.Equal(t, "find", cmd.Name())
	require.NotNil(t, cmd.Flag("query"))
	require.NotNil(t, cmd.Flag("url"))
	require.NotNil(t, cmd.Flag("limit"))

	// Check default limit.
	require.Equal(t, "20", cmd.Flag("limit").DefValue)

	// Short alias for query.
	require.Equal(t, "q", cmd.Flag("query").Shorthand)
}

func TestNewGhFindCmdErrorsWhenNoQueryOrURL(t *testing.T) {
	cmd := newGhFindCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "provide a query")
}

// ---------------------------------------------------------------------------
// newGhAppendCmd
// ---------------------------------------------------------------------------

func TestNewGhAppendCmdFlags(t *testing.T) {
	cmd := newGhAppendCmd()

	require.Equal(t, "append-record", cmd.Name())
	require.NotNil(t, cmd.Flag("file"))
	require.NotNil(t, cmd.Flag("url"))
	require.NotNil(t, cmd.Flag("date"))
	require.NotNil(t, cmd.Flag("des"))
	require.NotNil(t, cmd.Flag("topic"))
}
