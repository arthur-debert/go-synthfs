package operations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
	"github.com/arthur-debert/synthfs/pkg/synthfs/operations"
)

func TestBaseOperation_ValidateID(t *testing.T) {
	tests := []struct {
		name        string
		id          core.OperationID
		path        string
		expectError bool
		errorReason string
	}{
		{
			name:        "valid ID and path",
			id:          "test-op-1",
			path:        "/tmp/test",
			expectError: false,
		},
		{
			name:        "empty ID",
			id:          "",
			path:        "/tmp/test",
			expectError: true,
			errorReason: "operation ID cannot be empty",
		},
		{
			name:        "whitespace-only ID",
			id:          "   ",
			path:        "/tmp/test",
			expectError: true,
			errorReason: "operation ID cannot contain only whitespace",
		},
		{
			name:        "tab-only ID",
			id:          "\t\t",
			path:        "/tmp/test",
			expectError: true,
			errorReason: "operation ID cannot contain only whitespace",
		},
		{
			name:        "mixed whitespace ID",
			id:          " \t \n ",
			path:        "/tmp/test",
			expectError: true,
			errorReason: "operation ID cannot contain only whitespace",
		},
		{
			name:        "empty path",
			id:          "test-op-1",
			path:        "",
			expectError: true,
			errorReason: "path cannot be empty",
		},
		{
			name:        "both empty",
			id:          "",
			path:        "",
			expectError: true,
			errorReason: "operation ID cannot be empty", // ID is checked first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := operations.NewBaseOperation(tt.id, "test", tt.path)
			err := op.Validate(context.Background(), nil, nil)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				var validationErr *core.ValidationError
				if !errors.As(err, &validationErr) {
					t.Errorf("expected ValidationError but got %T", err)
					return
				}
				if validationErr.Reason != tt.errorReason {
					t.Errorf("expected reason %q but got %q", tt.errorReason, validationErr.Reason)
				}
				if validationErr.OperationID != tt.id {
					t.Errorf("expected operation ID %q but got %q", tt.id, validationErr.OperationID)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

