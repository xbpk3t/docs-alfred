package wiki

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSummaryFrontmatter(t *testing.T) {
	raw := `---
title: demo
date: 2026-06-06
source: wiki-inbox
batch_id: wiki-2026-06-06
total_urls: 2
succeeded: 1
failed: 1
---

## 2026-06-06

body
`

	result := parseSummaryFrontmatter(raw)

	require.NotNil(t, result)
	assert.Equal(t, "demo", result.fm.Title)
	assert.Equal(t, "wiki-2026-06-06", result.fm.BatchID)
	assert.Contains(t, result.body, "## 2026-06-06")
}

func TestParseSummaryFrontmatterBodyCanContainDelimiters(t *testing.T) {
	raw := `---
title: demo
date: 2026-06-06
---

before
---
after
`

	result := parseSummaryFrontmatter(raw)

	require.NotNil(t, result)
	assert.Contains(t, result.body, "before\n---\nafter")
}

func TestParseSummaryFrontmatterRequiresLeadingFrontmatter(t *testing.T) {
	raw := `intro
---
title: demo
---
`

	assert.Nil(t, parseSummaryFrontmatter(raw))
}
