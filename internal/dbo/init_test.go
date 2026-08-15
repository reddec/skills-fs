package dbo_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/reddec/skills-fs/internal/dbo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewFromFileMemoryIsEphemeral verifies the documented "file::memory:" form opens an
// in-memory database: it must be usable (single connection keeps it alive) and must not
// create literal files such as "file::memory:" / "-shm" / "-wal" in the working directory
// (a double "file:" URI prefix previously caused exactly that).
func TestNewFromFileMemoryIsEphemeral(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	q, err := dbo.NewFromFile("file::memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	ctx := context.Background()
	id, err := q.CreateSkill(ctx, dbo.CreateSkillParams{
		Name:        "mem-skill",
		Description: "lives in RAM",
		Body:        "# hi",
		Metadata:    "{}",
	})
	require.NoError(t, err)

	sk, err := q.GetSkill(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "mem-skill", sk.Name)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "ephemeral database must not create files")
}

// TestNewFromFilePlainPathOpens covers the default form: a plain filesystem path still
// gets the "file:" URI prefix and WAL sidecar files as before.
func TestNewFromFilePlainPathOpens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	q, err := dbo.NewFromFile(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close() })

	_, err = q.ListSkills(context.Background())
	require.NoError(t, err)
	_, err = os.Stat(path)
	assert.NoError(t, err, "file database must exist on disk")
}
