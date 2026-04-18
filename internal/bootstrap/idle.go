package bootstrap

import (
	"context"

	"go.uber.org/zap"
)

// RunIdle blocks until ctx is cancelled. It is used by skeleton worker-style binaries.
func RunIdle(ctx context.Context, role string, log *zap.Logger) error {
	if log == nil {
		return nil
	}

	log.Info("skeleton process idle (no business processors yet)", zap.String("role", role))
	<-ctx.Done()

	err := ctx.Err()
	if err != nil {
		log.Info("skeleton process stopping", zap.String("role", role), zap.Error(err))
	} else {
		log.Info("skeleton process stopping", zap.String("role", role))
	}
	return err
}
