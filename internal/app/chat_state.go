package app

import (
	"context"
	"time"

	"go.mau.fi/whatsmeow/types"
)

func (a *App) ArchiveChat(ctx context.Context, jid types.JID, archive bool) error {
	if err := a.wa.ArchiveChat(ctx, jid, archive, time.Time{}, nil); err != nil {
		return err
	}
	return a.db.SetChatArchived(jid.String(), archive)
}

func (a *App) PinChat(ctx context.Context, jid types.JID, pin bool) error {
	if err := a.wa.PinChat(ctx, jid, pin); err != nil {
		return err
	}
	return a.db.SetChatPinned(jid.String(), pin)
}

func (a *App) MuteChat(ctx context.Context, jid types.JID, mute bool, duration time.Duration) error {
	if err := a.wa.MuteChat(ctx, jid, mute, duration); err != nil {
		return err
	}
	var mu int64
	if mute {
		if duration <= 0 {
			mu = -1
		} else {
			mu = time.Now().Add(duration).Unix()
		}
	}
	return a.db.SetChatMutedUntil(jid.String(), mu)
}

func (a *App) MarkChatRead(ctx context.Context, jid types.JID, read bool) error {
	if err := a.wa.MarkChatAsRead(ctx, jid, read, time.Time{}, nil); err != nil {
		return err
	}
	return a.db.SetChatUnread(jid.String(), !read)
}
