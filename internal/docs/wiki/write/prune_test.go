package write

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pruneFixture = `## 2026-11-05

### Keep me

` + "```markdown" + `
URL: https://example.com/keep
Type: inbox
reason: no_topic_match
` + "```" + `

#### overview
Keep this entry.

### Remove me

` + "```markdown" + `
URL: https://example.com/remove?utm_source=newsletter
Type: inbox
reason: no_topic_match
` + "```" + `

#### overview
Remove this entry.

## 2026-11-06

### Keep the next one

` + "```markdown" + `
URL: https://example.com/keep-two
Type: inbox
reason: no_topic_match
` + "```" + `
`

func TestPruneManualReviewRemovesMatchingBlock(t *testing.T) {
	store := newReviewR2("uncat.md")
	store.data["uncat.md"] = pruneFixture

	result, err := PruneManualReview(context.Background(), store, []string{"https://example.com/remove"}, false)
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)

	got := store.content("uncat.md")
	assert.NotContains(t, got, "URL: https://example.com/remove")
	assert.NotContains(t, got, "Remove this entry.")
	assert.Contains(t, got, "URL: https://example.com/keep")
	assert.Contains(t, got, "Keep the next one")
	assert.Contains(t, got, "## 2026-11-05")
	assert.Contains(t, got, "## 2026-11-06")
}

func TestPruneManualReviewDryRunDoesNotUpload(t *testing.T) {
	store := newReviewR2("uncat.md")
	store.data["uncat.md"] = pruneFixture

	result, err := PruneManualReview(context.Background(), store, []string{"https://example.com/remove"}, true)
	require.NoError(t, err)
	require.Len(t, result.Removed, 1)
	assert.Equal(t, pruneFixture, store.content("uncat.md"))
}

func TestPruneManualReviewNoMatchLeavesContent(t *testing.T) {
	store := newReviewR2("uncat.md")
	store.data["uncat.md"] = pruneFixture

	result, err := PruneManualReview(context.Background(), store, []string{"https://example.com/absent"}, false)
	require.NoError(t, err)
	assert.Empty(t, result.Removed)
	assert.Equal(t, pruneFixture, store.content("uncat.md"))
}
