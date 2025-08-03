package synthfs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDryRunFS_WriteFile(t *testing.T) {
	dryRunFS := NewDryRunFS()

	err := dryRunFS.WriteFile("test.txt", []byte("hello"), 0644)
	assert.NoError(t, err)

	// Check that the file exists in the dry run fs
	content, err := dryRunFS.ReadFile("test.txt")
	assert.NoError(t, err)
	assert.Equal(t, []byte("hello"), content)
}

func TestDryRunFS_Mkdir(t *testing.T) {
	dryRunFS := NewDryRunFS()

	err := dryRunFS.Mkdir("testdir", 0755)
	assert.NoError(t, err)

	// Check that the dir exists in the dry run fs
	fi, err := dryRunFS.Stat("testdir")
	assert.NoError(t, err)
	assert.True(t, fi.IsDir())
}

func TestDryRunFS_Remove(t *testing.T) {
	dryRunFS := NewDryRunFS()
	err := dryRunFS.WriteFile("test.txt", []byte("hello"), 0644)
	assert.NoError(t, err)

	err = dryRunFS.Remove("test.txt")
	assert.NoError(t, err)

	// Check that the file does NOT exist in the dry run fs
	_, err = dryRunFS.Stat("test.txt")
	assert.Error(t, err)
}
