package operations

import (
	"context"
	"time"

	"github.com/arthur-debert/synthfs/pkg/synthfs/core"
)

// executeWithEvents is a helper that wraps operation execution with event handling
func executeWithEvents(op Operation, ctx context.Context, execCtx *core.ExecutionContext, fsys interface{},
	executeFunc func(context.Context, interface{}) error) error {

	// Emit operation started event
	if execCtx.EventBus != nil {
		startEvent := core.NewOperationStartedEvent(
			op.ID(),
			op.Describe().Type,
			op.Describe().Path,
			op.Describe().Details,
		)
		execCtx.EventBus.PublishAsync(ctx, startEvent)
	}

	// Execute the operation and measure duration
	startTime := time.Now()

	execCtx.Logger.Trace().
		Str("op_id", string(op.ID())).
		Str("op_type", op.Describe().Type).
		Str("path", op.Describe().Path).
		Msg("executing operation")

	err := executeFunc(ctx, fsys)
	duration := time.Since(startTime)

	// Emit completion or failure event
	if execCtx.EventBus != nil {
		if err != nil {
			failEvent := core.NewOperationFailedEvent(
				op.ID(),
				op.Describe().Type,
				op.Describe().Path,
				op.Describe().Details,
				err,
				duration,
			)
			execCtx.EventBus.PublishAsync(ctx, failEvent)
		} else {
			completeEvent := core.NewOperationCompletedEvent(
				op.ID(),
				op.Describe().Type,
				op.Describe().Path,
				op.Describe().Details,
				duration,
			)
			execCtx.EventBus.PublishAsync(ctx, completeEvent)
		}
	}

	if err != nil {
		execCtx.Logger.Trace().
			Str("op_id", string(op.ID())).
			Str("op_type", op.Describe().Type).
			Str("path", op.Describe().Path).
			Dur("duration", duration).
			Err(err).
			Msg("operation failed")
	} else {
		execCtx.Logger.Trace().
			Str("op_id", string(op.ID())).
			Str("op_type", op.Describe().Type).
			Str("path", op.Describe().Path).
			Dur("duration", duration).
			Msg("operation completed")
	}

	return err
}
