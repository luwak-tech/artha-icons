package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStorage_InitAndSave(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "mapping.json")

	// 1. Init new storage (file doesn't exist yet)
	store, err := New(filePath)
	assert.NoError(t, err)
	assert.NotNil(t, store)

	// 2. Add an item
	isin := "INE123A01012"
	filename := "INE123A01012.svg"
	store.Add(isin, filename)

	// 3. Verify it's in memory
	assert.True(t, store.Has(isin))

	// 4. Save to disk
	err = store.Save()
	assert.NoError(t, err)

	// 5. Re-init to verify loading from disk
	store2, err := New(filePath)
	assert.NoError(t, err)
	assert.True(t, store2.Has(isin))
	assert.Equal(t, filename, store2.Mapping[isin])
}

func TestStorage_EmptyLoad(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "empty.json")

	// Create empty file
	err := os.WriteFile(filePath, []byte(""), 0644)
	assert.NoError(t, err)

	store, err := New(filePath)
	assert.NoError(t, err)
	assert.Empty(t, store.Mapping)
}
