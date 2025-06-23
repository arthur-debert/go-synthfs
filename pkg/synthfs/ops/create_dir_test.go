package ops_test

import (
	"context"
	"errors"
	"io/fs"
	"syscall"
	"testing"

	"go-synthfs/pkg/synthfs"
	"go-synthfs/pkg/synthfs/internal/testutil"
	"go-synthfs/pkg/synthfs/ops"
)

func TestCreateDirOperation_Execute(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()

	dirPath := "testdir"
	dirMode := fs.FileMode(0755) | fs.ModeDir // MkdirAll expects ModeDir

	op := ops.NewCreateDir(dirPath, dirMode)

	err := op.Execute(ctx, mfs)
	if err != nil {
		t.Fatalf("Execute() error = %v, wantErr %v", err, false)
	}

	info, err := mfs.Stat(dirPath)
	if err != nil {
		t.Fatalf("mfs.Stat() error = %v for path %s", err, dirPath)
	}
	if !info.IsDir() {
		t.Errorf("Stat().IsDir() = false, want true for path %s", dirPath)
	}
	// Check permission bits (MockFS might add fs.ModeDir itself)
	if info.Mode()&fs.ModePerm != dirMode&fs.ModePerm {
		t.Errorf("Stat().Mode() perm = %v, want %v", info.Mode()&fs.ModePerm, dirMode&fs.ModePerm)
	}

	// Test MkdirAll behavior (creating parents)
	nestedDirPath := "parent/child/grandchild"
	nestedDirMode := fs.FileMode(0750) | fs.ModeDir
	opNested := ops.NewCreateDir(nestedDirPath, nestedDirMode)
	err = opNested.Execute(ctx, mfs)
	if err != nil {
		t.Fatalf("Execute() for nested dir error = %v", err)
	}
	infoNested, err := mfs.Stat(nestedDirPath)
	if err != nil {
		t.Fatalf("mfs.Stat() for nested dir error = %v", err)
	}
	if !infoNested.IsDir() {
		t.Errorf("Stat().IsDir() for nested dir = false, want true")
	}
	if infoNested.Mode()&fs.ModePerm != nestedDirMode&fs.ModePerm {
		t.Errorf("Stat().Mode() perm for nested dir = %v, want %v", infoNested.Mode()&fs.ModePerm, nestedDirMode&fs.ModePerm)
	}
	// Check if parent was created
	infoParent, err := mfs.Stat("parent/child")
	if err != nil {
		t.Fatalf("mfs.Stat() for parent dir error = %v", err)
	}
	if !infoParent.IsDir() {
		t.Errorf("Stat().IsDir() for parent dir = false, want true")
	}
}

func TestCreateDirOperation_Rollback(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		setupFS     func(mfs *testutil.MockFS) // Setup initial state
		path        string
		mode        fs.FileMode
		addPostOp   func(mfs *testutil.MockFS, path string) error // Add something after Execute, before Rollback
		wantErr     bool
		verifyState func(t *testing.T, mfs *testutil.MockFS, path string)
	}{
		{
			name: "simple rollback",
			path: "simple_rollback_dir",
			mode: 0755 | fs.ModeDir,
			verifyState: func(t *testing.T, mfs *testutil.MockFS, path string) {
				_, err := mfs.Stat(path)
				if !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("Expected dir %s to be removed, but Stat error is %v", path, err)
				}
			},
		},
		{
			name: "rollback of MkdirAll (only target dir)",
			path: "parent/child_for_rollback",
			mode: 0755 | fs.ModeDir,
			verifyState: func(t *testing.T, mfs *testutil.MockFS, path string) {
				_, err := mfs.Stat(path)
				if !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("Expected dir %s to be removed, but Stat error is %v", path, err)
				}
				// Parent should still exist as CreateDirOperation.Rollback is not recursive for parents
				_, errParent := mfs.Stat("parent")
				if errParent != nil {
					t.Errorf("Expected parent dir 'parent' to still exist, but Stat error is %v", errParent)
				}
			},
		},
		{
			name: "rollback of non-empty directory (should fail)",
			path: "non_empty_dir_rollback",
			mode: 0755 | fs.ModeDir,
			addPostOp: func(mfs *testutil.MockFS, path string) error {
				return mfs.WriteFile(path+"/file.txt", []byte("data"), 0644)
			},
			wantErr: true, // Rollback of non-empty dir using os.Remove semantics should fail
			verifyState: func(t *testing.T, mfs *testutil.MockFS, path string) {
				_, err := mfs.Stat(path) // Directory should still exist
				if err != nil {
					t.Errorf("Expected dir %s to still exist after failed rollback, but Stat error is %v", path, err)
				}
				_, errFile := mfs.Stat(path + "/file.txt") // File should also still exist
				if errFile != nil {
					t.Errorf("Expected file %s/file.txt to still exist, but Stat error is %v", path, errFile)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mfs := testutil.NewMockFS()
			if tt.setupFS != nil {
				tt.setupFS(mfs)
			}

			op := ops.NewCreateDir(tt.path, tt.mode)

			if errExecute := op.Execute(ctx, mfs); errExecute != nil {
				t.Fatalf("Execute() failed: %v", errExecute)
			}

			if tt.addPostOp != nil {
				if errPost := tt.addPostOp(mfs, tt.path); errPost != nil {
					t.Fatalf("addPostOp() failed: %v", errPost)
				}
			}

			errRollback := op.Rollback(ctx, mfs)
			if (errRollback != nil) != tt.wantErr {
				t.Errorf("Rollback() error = %v, wantErr %v", errRollback, tt.wantErr)
			}
			if tt.wantErr && errRollback != nil {
				// Check for specific error if directory not empty (syscall.ENOTEMPTY)
				// Note: The current CreateDirOperation.Rollback wraps the error.
				// We'd need to unwrap or check string if we want to be super specific here.
				// For now, just checking if an error occurred as expected is enough.
				var pathErr *fs.PathError
				if errors.As(errRollback, &pathErr) { // Our mockFS returns PathError with ENOTEMPTY
					if !errors.Is(pathErr.Err, syscall.ENOTEMPTY) {
						t.Logf("Rollback() error %v, expected specific error type like ENOTEMPTY", errRollback)
					}
				} else {
					// The synthfs.CreateDirOperation.Rollback wraps it further.
					// So, we might not get PathError directly.
					// A string check might be too brittle.
					// t.Logf("Rollback error: %v", errRollback)
				}
			}

			if tt.verifyState != nil {
				tt.verifyState(t, mfs, tt.path)
			}
		})
	}
}

func TestCreateDirOperation_Validate(t *testing.T) {
	ctx := context.Background()
	mfs := testutil.NewMockFS()

	tests := []struct {
		name    string
		op      *ops.CreateDirOperation
		wantErr bool
	}{
		{
			name:    "valid operation",
			op:      ops.NewCreateDir("valid_dir", 0755|fs.ModeDir),
			wantErr: false,
		},
		{
			name:    "valid operation (perms only)",
			op:      ops.NewCreateDir("valid_dir_perms_only", 0755), // MkdirAll adds ModeDir
			wantErr: false, // Validate should be fine, Execute handles ModeDir
		},
		{
			name:    "empty path",
			op:      ops.NewCreateDir("", 0755|fs.ModeDir),
			wantErr: true,
		},
		{
			name:    "invalid mode (non-permission bits, excluding ModeDir)",
			// Example: fs.ModeSymlink is not a perm bit and not ModeDir
			op:      ops.NewCreateDir("invalid_mode_dir", fs.ModeSymlink|0755),
			wantErr: true,
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

func TestCreateDirOperation_Describe(t *testing.T) {
	dirPath := "desc_dir"
	dirMode := fs.FileMode(0755) | fs.ModeDir
	op := ops.NewCreateDir(dirPath, dirMode)

	desc := op.Describe()
	if desc.Type != "create_dir" {
		t.Errorf("Describe().Type = %s, want create_dir", desc.Type)
	}
	if desc.Path != dirPath {
		t.Errorf("Describe().Path = %s, want %s", desc.Path, dirPath)
	}
	if modeStr, ok := desc.Details["mode"].(string); !ok || modeStr != dirMode.String() {
		t.Errorf("Describe().Details['mode'] = %v, want %v", desc.Details["mode"], dirMode.String())
	}
}

func TestCreateDirOperation_WithID_WithDependency(t *testing.T) {
	op := ops.NewCreateDir("testdir", 0755)
	customID := synthfs.OperationID("custom-dir-id")
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
