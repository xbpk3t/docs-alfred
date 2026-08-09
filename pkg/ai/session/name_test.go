package session

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func writeLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	var data string
	for _, l := range lines {
		data += l + "\n"
	}
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
}

func TestSessionNameFromCC_LatestEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	writeLines(t, path,
		`{"type":"ai-title","aiTitle":"旧标题","sessionId":"s1"}`,
		`{"type":"user","message":{"role":"user","content":"继续"}}`,
		`{"type":"ai-title","aiTitle":"新标题","sessionId":"s1"}`,
	)

	name, err := SessionNameFromCC(path)
	require.NoError(t, err)
	assert.Equal(t, "新标题", name)
}

func TestSessionNameFromCC_NoTitle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	writeLines(t, path,
		`{"type":"user","message":{"role":"user","content":"hi"}}`,
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"ok"}]}}`,
	)

	_, err := SessionNameFromCC(path)
	require.ErrorIs(t, err, ErrNoSessionName)
}

func TestSessionNameFromCC_MissingFile(t *testing.T) {
	_, err := SessionNameFromCC(filepath.Join(t.TempDir(), "nope.jsonl"))
	require.Error(t, err)
}

func TestSessionNameFromCodex_TitleWins(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.sqlite")
	db, err := sql.Open("sqlite", statePath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE threads (
		id TEXT PRIMARY KEY,
		rollout_path TEXT NOT NULL,
		title TEXT NOT NULL,
		first_user_message TEXT NOT NULL DEFAULT ''
	)`)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO threads (id, rollout_path, title, first_user_message) VALUES (?, ?, ?, ?)`,
		"t1", "/tmp/rollout.jsonl", "用户改名后的标题", "原始首条消息",
	)
	require.NoError(t, err)

	name, err := SessionNameFromCodex(statePath, "t1")
	require.NoError(t, err)
	assert.Equal(t, "用户改名后的标题", name)
}

func TestSessionNameFromCodex_FallsBackToFirstUserMessage(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.sqlite")
	db, err := sql.Open("sqlite", statePath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE threads (
		id TEXT PRIMARY KEY,
		rollout_path TEXT NOT NULL,
		title TEXT NOT NULL,
		first_user_message TEXT NOT NULL DEFAULT ''
	)`)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO threads (id, rollout_path, title, first_user_message) VALUES (?, ?, ?, ?)`,
		"t1", "/tmp/rollout.jsonl", "", "首条消息内容",
	)
	require.NoError(t, err)

	name, err := SessionNameFromCodex(statePath, "t1")
	require.NoError(t, err)
	assert.Equal(t, "首条消息内容", name)
}

func TestSessionNameFromCodex_NoRows(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.sqlite")
	db, err := sql.Open("sqlite", statePath)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	_, err = db.Exec(`CREATE TABLE threads (
		id TEXT PRIMARY KEY,
		rollout_path TEXT NOT NULL,
		title TEXT NOT NULL,
		first_user_message TEXT NOT NULL DEFAULT ''
	)`)
	require.NoError(t, err)

	_, err = SessionNameFromCodex(statePath, "missing")
	require.Error(t, err)
}

func TestSessionNameFromCodex_MissingState(t *testing.T) {
	_, err := SessionNameFromCodex(filepath.Join(t.TempDir(), "nope.sqlite"), "t1")
	require.Error(t, err)
}
