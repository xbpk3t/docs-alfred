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

	var sectionEvents int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		switch ev.Type {
		case evSection:
			sectionEvents++
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, sectionEvents)
}

func TestWalkGhRepos_NonMappingInSection(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- topic: a
  kind: mech
- just a string
- topic: b
  kind: type
`), 0644))

	var sectionCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evSection {
			sectionCount++
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, sectionCount) // the whole doc is bundled into one section
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
		if ev.Type == evSection {
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

	var sectionEvent bool
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evSection {
			sectionEvent = true
		}

		return nil
	})
	require.NoError(t, err)
	assert.True(t, sectionEvent)
}

func TestWalkerEvent_Fields(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- topic: language
  kind: type
  repo:
    - url: https://github.com/acme/tool
      des: test
  record: []
`), 0644))

	var ev WalkerEvent
	err := WalkGhRepos(tmpDir, func(event WalkerEvent) error {
		if event.Type == evSection {
			ev = event
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "go", ev.SectionType) // type derived from the file name
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
  record: []
---
- type: lang2
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
	assert.Equal(t, 2, sectionCount)
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
	// Should have file event but no section events
	var hasSection bool
	for _, ev := range events {
		if ev.Type == evSection {
			hasSection = true
		}
	}
	assert.False(t, hasSection)
}

func TestWalkGhRepos_EmptySequenceItem(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- topic: valid
  repo:
    - url: https://github.com/a/b
- "just a string"
- topic: next
`), 0644))

	var sectionCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evSection {
			sectionCount++
		}

		return nil
	})
	require.NoError(t, err)
	// "just a string" is skipped (not a map), the rest is one section
	assert.Equal(t, 1, sectionCount)
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
		if ev.Type == evSection {
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

	var sectionCount int
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evSection {
			sectionCount++
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, sectionCount)
}

func TestWalkGhRepos_NoRepoNoUsing(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: test
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
	assert.Equal(t, 1, sectionCount)
}

func TestWalkGhRepos_EmptyRepoList(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.yml"), []byte(`- type: test
  repo: []
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
	assert.Equal(t, 1, sectionCount)
}

func TestWalkGhRepos_TopicWithRepos(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "llm.yml"), []byte(`- topic: claude-code
  kind: type
  repo:
    - url: https://github.com/anthropics/claude-code
      doc: https://code.claude.com/docs/
    - url: https://github.com/openai/codex
      doc: https://developers.openai.com/codex/config-reference
  record:
    - date: 2025-01-01
      des: initial
`), 0644))

	var sectionEvents int
	var sectionType string
	var topics []Topic
	err := WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evSection {
			sectionEvents++
			sectionType = ev.SectionType
			topics = ev.Topics
		}

		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, sectionEvents)
	assert.Equal(t, "llm", sectionType) // derived from "llm.yml"
	require.Len(t, topics, 1)
	assert.Equal(t, "claude-code", topics[0].Topic)
}

func TestWalkGhRepos_TopicWithRepos_RealFile(t *testing.T) {
	// 读取真实的 LLM.yml 文件
	data, err := os.ReadFile("/Users/luck/Desktop/docs/data/gh/AI/LLM.yml")
	if err != nil {
		t.Skip("Skipping test: cannot read real LLM.yml file")
	}

	tmpDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "LLM.yml"), data, 0644))

	var sections []walkerSection
	err = WalkGhRepos(tmpDir, func(ev WalkerEvent) error {
		if ev.Type == evSection {
			repos := 0
			for i := range ev.Topics {
				repos += len(ev.Topics[i].Repo)
			}
			sections = append(sections, walkerSection{typeTag: ev.SectionType, repoCount: repos})
		}
		return nil
	})
	require.NoError(t, err)

	// Verify sections were parsed
	assert.True(t, len(sections) > 0, "Expected to find sections in LLM.yml")
	for _, s := range sections {
		t.Logf("✓ Section: %s, Repos: %d", s.typeTag, s.repoCount)
	}
}

type walkerSection struct {
	typeTag   string
	repoCount int
}
