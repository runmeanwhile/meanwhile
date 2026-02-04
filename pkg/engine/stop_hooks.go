package engine

import (
	"context"
	"errors"

	"github.com/runmeanwhile/meanwhile/pkg/event"
	"github.com/runmeanwhile/meanwhile/pkg/hook"
)

// ErrStopBlocked indicates session shutdown was blocked by a hook.
var ErrStopBlocked = errors.New("stop blocked by hook")

func (s *Session) applyStopHooks(ctx context.Context, reason hook.StopReason) error {
	if s.engine == nil || s.engine.hooks == nil {
		return nil
	}

	meta := hook.SessionMeta{SessionID: s.id}
	if s.protocol != nil {
		meta.ProtocolID = s.protocol.ID()
	}

	for _, h := range s.engine.hooks.StopHooks() {
		decision, err := h.OnStop(ctx, meta, reason)
		if err != nil {
			return err
		}
		if decision == hook.Block {
			_ = s.Emit(event.New(event.HookBlocked, s.id, map[string]any{"hook": h.ID()}))
			return ErrStopBlocked
		}
	}

	return nil
}
