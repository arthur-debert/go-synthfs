package synthfs_test

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs"
	"github.com/arthur-debert/synthfs/pkg/synthfs/testutil"
)

func TestReadFile_Basic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SynthFS does not officially support Windows")
	}

	t.Run("read text file", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create test file
		testContent := "Hello, World!\nThis is a test file."
		testFile := "test.txt"
		err := fs.WriteFile(testFile, []byte(testContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Read file with operation
		op := sfs.ReadFile(testFile)
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("ReadFile operation failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("ReadFile operation did not succeed: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify content was captured
		content := synthfs.GetOperationOutput(op, "content")
		if content != testContent {
			t.Errorf("Expected content %q, got %q", testContent, content)
		}

		// Verify metadata was captured
		size := synthfs.GetOperationOutputValue(op, "size")
		if size == nil {
			t.Error("Size metadata was not captured")
		}

		modTime := synthfs.GetOperationOutputValue(op, "modTime")
		if modTime == nil {
			t.Error("ModTime metadata was not captured")
		}
	})

	t.Run("read empty file", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create empty file
		testFile := "empty.txt"
		err := fs.WriteFile(testFile, []byte(""), 0644)
		if err != nil {
			t.Fatalf("Failed to create empty file: %v", err)
		}

		// Read empty file
		op := sfs.ReadFile(testFile)
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("ReadFile operation failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("ReadFile operation did not succeed: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify empty content
		content := synthfs.GetOperationOutput(op, "content")
		if content != "" {
			t.Errorf("Expected empty content, got %q", content)
		}

		// Verify size is 0
		size := synthfs.GetOperationOutputValue(op, "size")
		if sizeInt, ok := size.(int64); !ok || sizeInt != 0 {
			t.Errorf("Expected size 0, got %v", size)
		}
	})

	t.Run("read with explicit ID", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create test file
		testContent := "Test content for explicit ID"
		testFile := "explicit_id.txt"
		err := fs.WriteFile(testFile, []byte(testContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Read file with explicit ID
		op := sfs.ReadFileWithID("custom_read_id", testFile)
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("ReadFile operation failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("ReadFile operation did not succeed: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify content was captured
		content := synthfs.GetOperationOutput(op, "content")
		if content != testContent {
			t.Errorf("Expected content %q, got %q", testContent, content)
		}

		// Verify operation ID
		if op.ID() != "custom_read_id" {
			t.Errorf("Expected ID 'custom_read_id', got %q", op.ID())
		}
	})
}

func TestReadFile_ErrorHandling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SynthFS does not officially support Windows")
	}

	t.Run("nonexistent file", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Try to read nonexistent file
		op := sfs.ReadFile("nonexistent.txt")
		result, err := synthfs.Run(context.Background(), fs, op)
		
		if err == nil {
			t.Error("Expected error when reading nonexistent file")
		}

		if result.Success {
			t.Error("Operation should have failed for nonexistent file")
		}
	})

	t.Run("try to read directory", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create directory
		err := fs.MkdirAll("testdir", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Try to read directory as file
		op := sfs.ReadFile("testdir")
		result, err := synthfs.Run(context.Background(), fs, op)
		
		if err == nil {
			t.Error("Expected error when reading directory as file")
		}

		if result.Success {
			t.Error("Operation should have failed when reading directory")
		}

		// Check error message
		if !strings.Contains(err.Error(), "cannot read directory as file") {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})
}

func TestChecksum_Basic(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SynthFS does not officially support Windows")
	}

	t.Run("md5 checksum", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create test file with known content
		testContent := "Hello, World!"
		testFile := "test_md5.txt"
		err := fs.WriteFile(testFile, []byte(testContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Calculate MD5 checksum
		op := sfs.Checksum(testFile, synthfs.MD5)
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Checksum operation failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Checksum operation did not succeed: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify MD5 checksum (expected: 65a8e27d8879283831b664bd8b7f0ad4)
		md5Hash := synthfs.GetOperationOutput(op, "md5")
		expectedMD5 := "65a8e27d8879283831b664bd8b7f0ad4"
		if md5Hash != expectedMD5 {
			t.Errorf("Expected MD5 %s, got %s", expectedMD5, md5Hash)
		}

		// Verify algorithm was stored
		algorithm := synthfs.GetOperationOutput(op, "algorithm")
		if algorithm != "md5" {
			t.Errorf("Expected algorithm 'md5', got %s", algorithm)
		}

		// Verify metadata was captured
		size := synthfs.GetOperationOutputValue(op, "size")
		if size == nil {
			t.Error("Size metadata was not captured")
		}

		modTime := synthfs.GetOperationOutputValue(op, "modTime")
		if modTime == nil {
			t.Error("ModTime metadata was not captured")
		}
	})

	t.Run("sha1 checksum", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create test file
		testContent := "Hello, World!"
		testFile := "test_sha1.txt"
		err := fs.WriteFile(testFile, []byte(testContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Calculate SHA1 checksum
		op := sfs.Checksum(testFile, synthfs.SHA1)
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Checksum operation failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Checksum operation did not succeed: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify SHA1 checksum (expected: 0a0a9f2a6772942557ab5355d76af442f8f65e01)
		sha1Hash := synthfs.GetOperationOutput(op, "sha1")
		expectedSHA1 := "0a0a9f2a6772942557ab5355d76af442f8f65e01"
		if sha1Hash != expectedSHA1 {
			t.Errorf("Expected SHA1 %s, got %s", expectedSHA1, sha1Hash)
		}

		// Verify algorithm was stored
		algorithm := synthfs.GetOperationOutput(op, "algorithm")
		if algorithm != "sha1" {
			t.Errorf("Expected algorithm 'sha1', got %s", algorithm)
		}
	})

	t.Run("sha256 checksum", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create test file
		testContent := "Hello, World!"
		testFile := "test_sha256.txt"
		err := fs.WriteFile(testFile, []byte(testContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Calculate SHA256 checksum
		op := sfs.Checksum(testFile, synthfs.SHA256)
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Checksum operation failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Checksum operation did not succeed: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify SHA256 checksum (expected: dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f)
		sha256Hash := synthfs.GetOperationOutput(op, "sha256")
		expectedSHA256 := "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"
		if sha256Hash != expectedSHA256 {
			t.Errorf("Expected SHA256 %s, got %s", expectedSHA256, sha256Hash)
		}
	})

	t.Run("sha512 checksum", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create test file
		testContent := "Hello, World!"
		testFile := "test_sha512.txt"
		err := fs.WriteFile(testFile, []byte(testContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Calculate SHA512 checksum
		op := sfs.Checksum(testFile, synthfs.SHA512)
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Checksum operation failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Checksum operation did not succeed: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify SHA512 checksum exists and is correct length
		sha512Hash := synthfs.GetOperationOutput(op, "sha512")
		expectedSHA512 := "374d794a95cdcfd8b35993185fef9ba368f160d8daf432d08ba9f1ed1e5abe6cc69291e0fa2fe0006a52570ef18c19def4e617c33ce52ef0a6e5fbe318cb0387"
		if sha512Hash != expectedSHA512 {
			t.Errorf("Expected SHA512 %s, got %s", expectedSHA512, sha512Hash)
		}

		// Verify it's 128 hex characters (512 bits / 4 bits per hex char)
		if len(sha512Hash) != 128 {
			t.Errorf("Expected SHA512 hash length 128, got %d", len(sha512Hash))
		}
	})

	t.Run("checksum with explicit ID", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create test file
		testContent := "Test content for explicit ID"
		testFile := "explicit_checksum.txt"
		err := fs.WriteFile(testFile, []byte(testContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Calculate checksum with explicit ID
		op := sfs.ChecksumWithID("custom_checksum_id", testFile, synthfs.MD5)
		result, err := synthfs.Run(context.Background(), fs, op)
		if err != nil {
			t.Fatalf("Checksum operation failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Checksum operation did not succeed: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify checksum was captured
		md5Hash := synthfs.GetOperationOutput(op, "md5")
		if md5Hash == "" {
			t.Error("MD5 checksum was not captured")
		}

		// Verify operation ID
		if op.ID() != "custom_checksum_id" {
			t.Errorf("Expected ID 'custom_checksum_id', got %q", op.ID())
		}
	})
}

func TestChecksum_ErrorHandling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SynthFS does not officially support Windows")
	}

	t.Run("nonexistent file", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Try to checksum nonexistent file
		op := sfs.Checksum("nonexistent.txt", synthfs.MD5)
		result, err := synthfs.Run(context.Background(), fs, op)
		
		if err == nil {
			t.Error("Expected error when checksumming nonexistent file")
		}

		if result.Success {
			t.Error("Operation should have failed for nonexistent file")
		}
	})

	t.Run("try to checksum directory", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create directory
		err := fs.MkdirAll("testdir", 0755)
		if err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}

		// Try to checksum directory
		op := sfs.Checksum("testdir", synthfs.MD5)
		result, err := synthfs.Run(context.Background(), fs, op)
		
		if err == nil {
			t.Error("Expected error when checksumming directory")
		}

		if result.Success {
			t.Error("Operation should have failed when checksumming directory")
		}

		// Check error message
		if !strings.Contains(err.Error(), "cannot checksum directory") {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})
}

func TestFileOperations_InPipeline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SynthFS does not officially support Windows")
	}

	t.Run("read and checksum in pipeline", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create test content
		testContent := "This is test content for pipeline processing!"
		testFile := "pipeline_test.txt"

		// Run pipeline that creates file, reads it, and checksums it
		readOp := sfs.ReadFile(testFile)
		checksumOp := sfs.Checksum(testFile, synthfs.SHA256)
		
		result, err := synthfs.Run(context.Background(), fs,
			sfs.CreateFile(testFile, []byte(testContent), 0644),
			readOp,
			checksumOp,
		)

		if err != nil {
			t.Fatalf("Pipeline failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Pipeline did not succeed: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify read operation captured content
		content := synthfs.GetOperationOutput(readOp, "content")
		if content != testContent {
			t.Errorf("Expected content %q, got %q", testContent, content)
		}

		// Verify checksum operation calculated hash
		hash := synthfs.GetOperationOutput(checksumOp, "sha256")
		if hash == "" {
			t.Error("SHA256 checksum was not calculated")
		}

		// Verify both operations captured size metadata
		readSize := synthfs.GetOperationOutputValue(readOp, "size")
		checksumSize := synthfs.GetOperationOutputValue(checksumOp, "size")
		
		if readSize != checksumSize {
			t.Errorf("Size mismatch between read (%v) and checksum (%v)", readSize, checksumSize)
		}

		expectedSize := int64(len(testContent))
		if readSize != expectedSize {
			t.Errorf("Expected size %d, got %v", expectedSize, readSize)
		}
	})

	t.Run("multiple checksums of same file", func(t *testing.T) {
		helper := testutil.NewRealFSTestHelper(t)
		fs := helper.FileSystem()
		sfs := synthfs.New()

		// Create test file
		testContent := "Content for multiple checksums"
		testFile := "multi_checksum.txt"

		// Calculate multiple checksums with explicit IDs to avoid collision
		md5Op := sfs.ChecksumWithID("md5_checksum", testFile, synthfs.MD5)
		sha1Op := sfs.ChecksumWithID("sha1_checksum", testFile, synthfs.SHA1)
		sha256Op := sfs.ChecksumWithID("sha256_checksum", testFile, synthfs.SHA256)

		result, err := synthfs.Run(context.Background(), fs,
			sfs.CreateFile(testFile, []byte(testContent), 0644),
			md5Op,
			sha1Op,
			sha256Op,
		)

		if err != nil {
			t.Fatalf("Pipeline failed: %v", err)
		}

		if !result.Success {
			t.Fatalf("Pipeline did not succeed: %v", (func() string { if len(result.Errors) > 0 { return result.Errors[0].Error() } else { return "<no error>" } })())
		}

		// Verify all checksums were calculated
		md5Hash := synthfs.GetOperationOutput(md5Op, "md5")
		sha1Hash := synthfs.GetOperationOutput(sha1Op, "sha1")
		sha256Hash := synthfs.GetOperationOutput(sha256Op, "sha256")

		if md5Hash == "" {
			t.Error("MD5 checksum was not calculated")
		}
		if sha1Hash == "" {
			t.Error("SHA1 checksum was not calculated")
		}
		if sha256Hash == "" {
			t.Error("SHA256 checksum was not calculated")
		}

		// Verify all different
		if md5Hash == sha1Hash || md5Hash == sha256Hash || sha1Hash == sha256Hash {
			t.Error("Different hash algorithms should produce different results")
		}

		// Verify expected lengths
		if len(md5Hash) != 32 {
			t.Errorf("MD5 hash should be 32 chars, got %d", len(md5Hash))
		}
		if len(sha1Hash) != 40 {
			t.Errorf("SHA1 hash should be 40 chars, got %d", len(sha1Hash))
		}
		if len(sha256Hash) != 64 {
			t.Errorf("SHA256 hash should be 64 chars, got %d", len(sha256Hash))
		}
	})
}