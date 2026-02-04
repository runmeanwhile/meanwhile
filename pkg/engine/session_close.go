package engine

import (
	"context"

	"github.com/runmeanwhile/meanwhile/pkg/hook"
)

// CloseSession shuts down and removes a session.
func (e *Engine) CloseSession(ctx context.Context, sessionID string) error {
	e.mu.Lock()
	sess, ok := e.sessions[sessionID]
	if ok {
		delete(e.sessions, sessionID)
	}
	e.mu.Unlock()

	if !ok {
		return ErrSessionNotFound
	}

	if err := sess.protocol.Shutdown(ctx, sess); err != nil {
		return err
	}

	if e.timeoutScheduler != nil {
		for _, req := range sess.PendingRequests() {
			_ = e.timeoutScheduler.CancelTimeout(ctx, req.RequestID)
		}
	}

	if e.requestRegistry != nil {
		for _, req := range sess.PendingRequests() {
			_ = e.requestRegistry.Delete(ctx, req.RequestID)
		}
	}

	if err := sess.applyStopHooks(ctx, hook.StopReasonSessionClose); err != nil {
		return err
	}

	var memErr error
	if e.memoryAutomator != nil {
		if err := e.memoryAutomator.Capture(ctx, sess); err != nil {
			memErr = err
		}
	}

	sess.bus.Close()
	if e.sessionStore != nil {
		if stateStore, ok := e.sessionStore.(SessionStateStore); ok {
			if err := stateStore.DeleteState(ctx, sessionID); err != nil && err != ErrSessionNotFound {
				return err
			}
		}
		if err := e.sessionStore.Delete(ctx, sessionID); err != nil {
			return err
		}
	}
	return memErr
}
