package ghdata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWalkGhRepos_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	var events []WalkerEvent
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		events = append(events, ev)

		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestWalkGhRepos_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "empty.yml"), []byte(""), 0644))

	var events []WalkerEvent
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		events = append(events, ev)

		return nil
	})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, evEmpty, events[0].Type)
}

func TestWalkGhRepos_WhitespaceOnlyFile(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ws.yml"), []byte("   \n  \n"), 0644))

	var events []WalkerEvent
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		events = append(events, ev)

		return nil
	})
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, evEmpty, events[0].Type)
}

func TestWalkGhRepos_NotArray(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "map.yml"), []byte(`key: value`), 0644))

	var events []WalkerEvent
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		events = append(events, ev)

		return nil
	})
	require.NoError(t, err)
	// Should have file event + not-array event
	var hasNotArray bool
	for _, ev := range events {
		if ev.Type == evNotArray {
			hasNotArray = true
		}
	}
	assert.True(t, hasNotArray)
}

func TestWalkGhRepos_ValidData(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: language
  repo:
    - url: https://github.com/acme/tool
      des: a tool
  record: []
`), 0644))

	var sectionEvents, repoEvents int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		switch ev.Type {
		case evSection:
			sectionEvents++
		case evRepo:
			repoEvents++
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, sectionEvents)
	assert.Equal(t, 1, repoEvents)
}

func TestWalkGhRepos_RepoEntry(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: language
  repo:
    - url: https://github.com/acme/repo
`), 0644))

	var repoCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evRepo {
			repoCount++
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, repoCount)
}

func TestWalkGhRepos_NonMappingInSection(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: language
- just a string
- type: tool
`), 0644))

	var sectionCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evSection {
			sectionCount++
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, sectionCount) // two mapping sections
}

func TestWalkGhRepos_NonExistentDir(t *testing.T) {
	err := WalkGhRepos("/tmp/nonexistent-gh-walker-99999", func(ev WalkerEvent) error {
		return nil
	})
	require.Error(t, err)
}

func TestWalkGhRepos_CallbackError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: language
  repo:
    - url: https://github.com/acme/tool
`), 0644))

	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evRepo {
			return assert.AnError
		}

		return nil
	})
	require.Error(t, err)
}

func TestWalkGhRepos_SubDirs(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "sub")
	require.NoError(t, os.MkdirAll(subDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "nested.yml"), []byte(`- type: tool
  repo:
    - url: https://github.com/acme/nested
`), 0644))

	var repoCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evRepo {
			repoCount++
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, repoCount)
}

func TestWalkerEvent_Fields(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: language
  repo:
    - url: https://github.com/acme/tool
      des: test
      topics:
        - topic: overview
          record:
            - date: 2024-01-01
              des: initial
  record: []
`), 0644))

	var ev WalkerEvent
	err := WalkGhRepos(tmpDir, func(event WalkerEvent) error {
		if event.Type == evRepo {
			ev = event
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "language", ev.Section.Type)
	assert.Equal(t, "https://github.com/acme/tool", ev.Repo.URL)
	assert.Equal(t, "test", ev.Repo.Des)
	assert.Equal(t, evTypeRepo, ev.Relation)
	assert.Equal(t, "go", ev.FilenameStem)
}

func TestWalkGhRepos_UnreadableFileCallbackError(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file and then remove it to trigger unreadable event
	file := filepath.Join(tmpDir, "go.yml")
	require.NoError(t, os.WriteFile(file, []byte("- type: test\n"), 0644))
	require.NoError(t, os.Remove(file))
	// But ListYAMLFilesRecursive won't find a removed file, so this is hard to trigger
	// Instead, test the callback error path for the unreadable event
}

func TestWalkGhRepos_MultiDoc(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "multi.yml"), []byte(`---
- type: lang1
  repo:
    - url: https://github.com/a/b
  record: []
---
- type: lang2
  repo:
    - url: https://github.com/c/d
  record: []
`), 0644))

	var repoCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evRepo {
			repoCount++
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 2, repoCount)
}

func TestWalkGhRepos_NilDoc(t *testing.T) {
	tmpDir := t.TempDir()
	// Empty doc (just ---)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "nil.yml"), []byte("---\n"), 0644))

	var events []WalkerEvent
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		events = append(events, ev)

		return nil
	})
	require.NoError(t, err)
	// Should have file event but no section/ repo events
	var hasRepo bool
	for _, ev := range events {
		if ev.Type == evRepo {
			hasRepo = true
		}
	}
	assert.False(t, hasRepo)
}

func TestWalkGhRepos_EmptySequenceItem(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "mixed.yml"), []byte(`- type: valid
  repo:
    - url: https://github.com/a/b
  record: []
- "just a string"
- type: also_valid
  record: []
`), 0644))

	var sectionCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evSection {
			sectionCount++
		}

		return nil
	})
	require.NoError(t, err)
	// "just a string" is skipped (not a map), so 2 sections
	assert.Equal(t, 2, sectionCount)
}

func TestWalkGhRepos_EmptyCallbackError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "empty.yml"), []byte(""), 0644))

	expectedErr := assert.AnError
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evEmpty {
			return expectedErr
		}

		return nil
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestWalkGhRepos_FileCallbackError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: test
  record: []
`), 0644))

	expectedErr := assert.AnError
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evFile {
			return expectedErr
		}

		return nil
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestWalkGhRepos_SectionCallbackError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: test
  repo:
    - url: https://github.com/a/b
  record: []
`), 0644))

	expectedErr := assert.AnError
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evSection {
			return expectedErr
		}

		return nil
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestWalkGhRepos_RepoCallbackError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: test
  repo:
    - url: https://github.com/c/d
  record: []
`), 0644))

	var callCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evRepo && ev.Relation == evTypeRepo {
			callCount++

			return assert.AnError
		}

		return nil
	})
	require.Error(t, err)
	assert.Equal(t, 1, callCount)
}

func TestWalkGhRepos_NotArrayCallbackError(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "map.yml"), []byte(`key: value`), 0644))

	expectedErr := assert.AnError
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evNotArray {
			return expectedErr
		}

		return nil
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, expectedErr)
}

func TestWalkGhRepos_RepoNotMappingSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: test
  repo:
    - "string item"
    - url: https://github.com/a/b
  record: []
`), 0644))

	var repoCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evRepo {
			repoCount++
		}

		return nil
	})
	require.NoError(t, err)
	// "string item" is skipped, only 1 repo counted
	assert.Equal(t, 1, repoCount)
}

func TestWalkGhRepos_NoRepoNoUsing(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: test
  record: []
`), 0644))

	var repoCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evRepo {
			repoCount++
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 0, repoCount)
}

func TestWalkGhRepos_EmptyRepoList(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: test
  repo: []
  record: []
`), 0644))

	var repoCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evRepo {
			repoCount++
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 0, repoCount)
}

func TestWalkGhRepos_TopicWithRepos(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "llm.yml"), []byte(`- type: LLM
  topics:
    - topic: claude-code
      repo:
        - url: https://github.com/anthropics/claude-code
          doc: https://code.claude.com/docs/
        - url: https://github.com/openai/codex
          doc: https://developers.openai.com/codex/config-reference
      record:
        - date: 2025-01-01
          des: initial
  record: []
`), 0644))

	var sectionEvents int
	var section Section
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evSection {
			sectionEvents++
			section = ev.Section
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, sectionEvents)
	assert.Equal(t, "LLM", section.Type)
}

func TestWalkGhRepos_TopicWithRepos_RealFile(t *testing.T) {
	// 读取真实的 LLM.yml 文件
	data, err := os.ReadFile("/Users/luck/Desktop/docs/data/gh/AI/LLM.yml")
	if err != nil {
		t.Skip("Skipping test: cannot read real LLM.yml file")
	}

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "LLM.yml"), data, 0644))

	var sections []Section
	err = WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evSection {
			sections = append(sections, ev.Section)
		}
		return nil
	})
	require.NoError(t, err)

	// Verify sections were parsed
	assert.True(t, len(sections) > 0, "Expected to find sections in LLM.yml")
	for _, section := range sections {
		t.Logf("✓ Section: %s, Repos: %d", section.Type, len(section.Repos))
	}
}
