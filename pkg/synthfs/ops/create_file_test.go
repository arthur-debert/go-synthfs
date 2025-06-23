package ops_test

import (
	"context"
	"errors" // Consolidated import
	"io/fs"
	"testing"

	"go-synthfs/pkg/synthfs"
	"go-synthfs/pkg/synthfs/internal/testutil"
	"go-synthfs/pkg/synthfs/ops"
)

func TestCreateFileOperation_Execute(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()

	filePath := "test.txt"
	fileData := []byte("hello world")
	fileMode := fs.FileMode(0644)

	op := ops.NewCreateFile(filePath, fileData, fileMode)

	err := op.Execute(ctx, mfs)
	if err != nil {
		t.Fatalf("Execute() error = %v, wantErr %v", err, false)
	}

	data, err := mfs.ReadFile(filePath)
	if err != nil {
		t.Fatalf("mfs.ReadFile() error = %v", err)
	}
	if string(data) != string(fileData) {
		t.Errorf("ReadFile() data = %s, want %s", data, fileData)
	}

	info, err := mfs.Stat(filePath)
	if err != nil {
		t.Fatalf("mfs.Stat() error = %v", err)
	}
	if info.Mode() != fileMode {
		// The MockFS's Stat might not preserve exact original mode's non-permission bits
		// Let's check the permission part.
		if info.Mode()&fs.ModePerm != fileMode&fs.ModePerm {
			t.Errorf("Stat().Mode() perm = %v, want %v", info.Mode()&fs.ModePerm, fileMode&fs.ModePerm)
		}
	}
}

func TestCreateFileOperation_Rollback(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS() // Corrected to use testutil

	filePath := "rollback_test.txt"
	fileData := []byte("to be rolled back")
	fileMode := fs.FileMode(0644)

	op := ops.NewCreateFile(filePath, fileData, fileMode)

	if err := op.Execute(ctx, mfs); err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if _, err := mfs.Stat(filePath); err != nil {
		t.Fatalf("Stat() before rollback failed: %v", err)
	}

	err := op.Rollback(ctx, mfs)
	if err != nil {
		t.Fatalf("Rollback() error = %v, wantErr %v", err, false)
	}

	_, err = mfs.Stat(filePath)
	if err == nil {
		t.Errorf("Stat() after rollback succeeded, want error (file not exist)")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat() after rollback error = %v, want fs.ErrNotExist", err)
	}
}

func TestCreateFileOperation_Validate(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS() // Corrected to use testutil

	tests := []struct {
		name    string
		op      *ops.CreateFileOperation
		wantErr bool
	}{
		{
			name:    "valid operation",
			op:      ops.NewCreateFile("valid.txt", []byte("data"), 0644),
			wantErr: false,
		},
		{
			name:    "empty path",
			op:      ops.NewCreateFile("", []byte("data"), 0644),
			wantErr: true,
		},
		{
			name:    "invalid mode (non-permission bits)",
			// fs.FileMode can include type bits like ModeSymlink, ModeDevice etc.
			// Our Validate checks `op.mode&^fs.ModePerm != 0`
			// So, 07777 is fs.ModeSocket | fs.ModeSetgid | fs.ModeSetuid | fs.ModeSticky | 0777
			// (fs.ModeSocket | fs.ModeSetgid | fs.ModeSetuid | fs.ModeSticky) &^ 0777 is non-zero.
			op:      ops.NewCreateFile("invalid_mode.txt", []byte("data"), fs.ModeSocket|0777),
			wantErr: true,
		},
		{
			name:    "valid mode (permission bits only)",
			op:      ops.NewCreateFile("valid_mode.txt", []byte("data"), 0755),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.op.Validate(ctx, mfs)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateFileOperation_Describe(t *testing.T) {
	filePath := "desc.txt"
	fileData := []byte("description data")
	fileMode := fs.FileMode(0644)
	op := ops.NewCreateFile(filePath, fileData, fileMode)

	desc := op.Describe()
	if desc.Type != "create_file" {
		t.Errorf("Describe().Type = %s, want create_file", desc.Type)
	}
	if desc.Path != filePath {
		t.Errorf("Describe().Path = %s, want %s", desc.Path, filePath)
	}
	if size, ok := desc.Details["size"].(int); !ok || size != len(fileData) {
		t.Errorf("Describe().Details['size'] = %v, want %v", desc.Details["size"], len(fileData))
	}
}

func TestCreateFileOperation_WithID_WithDependency(t *testing.T) {
	op := ops.NewCreateFile("test.txt", []byte{}, 0644)
	customID := synthfs.OperationID("custom-create-id")
	depID := synthfs.OperationID("dep-id")

	op.WithID(customID).WithDependency(depID)

	if op.ID() != customID {
		t.Errorf("ID() = %s, want %s", op.ID(), customID)
	}
	deps := op.Dependencies()
	if len(deps) != 1 || deps[0] != depID {
		t.Errorf("Dependencies() = %v, want [%s]", deps, depID)
	}
}

// Removed var _ = errors.Is and duplicate import "errors"
// as errors is imported once at the top.
