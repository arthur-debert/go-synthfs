package operations

import (
	"context"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/filesystem"
)

// Operation represents a filesystem operation.
type Operation interface {
	// Core metadata
	ID() core.OperationID
	Describe() core.OperationDesc

	// Dependencies
	AddDependency(depID core.OperationID)
	Prerequisites() []core.Prerequisite

	// Item management - returns interface{} to avoid coupling to specific types
	GetItem() interface{}
	SetItem(item interface{})

	// Path management for operations like copy/move
	GetPaths() (src, dst string)
	SetPaths(src, dst string)

	// Checksum management - returns interface{} instead of ChecksumRecord
	GetChecksum(path string) interface{}
	GetAllChecksums() map[string]interface{}
	SetChecksum(path string, checksum interface{})

	// Description details
	SetDescriptionDetail(key string, value interface{})

	// Execution methods
	Execute(ctx context.Context, fsys filesystem.FileSystem) error
	Validate(ctx context.Context, fsys filesystem.FileSystem) error
	Rollback(ctx context.Context, fsys filesystem.FileSystem) error

	// ExecuteV2 for new execution context pattern
	ExecuteV2(ctx interface{}, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error
	ValidateV2(ctx interface{}, execCtx *core.ExecutionContext, fsys filesystem.FileSystem) error

	// Reverse operations
	ReverseOps(ctx context.Context, fsys filesystem.FileSystem, budget interface{}) ([]Operation, interface{}, error)
}

// ItemInterface represents a filesystem item to be created
type ItemInterface interface {
	Path() string
	Type() string
}

// ChecksumInterface represents a checksum record
type ChecksumInterface interface {
	GetPath() string
	GetMD5() string
	GetSize() int64
}
