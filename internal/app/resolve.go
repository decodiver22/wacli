package app

import (
	"context"
	"strings"

	"github.com/steipete/wacli/internal/store"
	"github.com/steipete/wacli/internal/wa"
	"go.mau.fi/whatsmeow/types"
)

// ResolveSenderNames fills in SenderName for messages where the sender is a LID
// (hidden user) and the name wasn't stored at sync time. It resolves LID→PN via
// the WhatsApp session store, then looks up the contact name for the phone number.
// Messages with an already-populated SenderName or a non-LID sender are skipped.
func (a *App) ResolveSenderNames(ctx context.Context, msgs []store.Message) {
	if len(msgs) == 0 {
		return
	}

	// Best-effort: open the WA session store for LID→PN lookups.
	if err := a.OpenWA(); err != nil {
		return
	}

	for i := range msgs {
		m := &msgs[i]
		if m.SenderName != "" || m.FromMe {
			continue
		}
		senderJID := strings.TrimSpace(m.SenderJID)
		if senderJID == "" {
			continue
		}
		if !strings.Contains(senderJID, "@lid") {
			continue
		}

		jid, err := types.ParseJID(senderJID)
		if err != nil {
			continue
		}

		pn, err := a.wa.GetPNForLID(ctx, jid)
		if err != nil || pn.IsEmpty() {
			continue
		}

		if info, err := a.wa.GetContact(ctx, pn); err == nil {
			if name := wa.BestContactName(info); name != "" {
				m.SenderName = name
				continue
			}
		}

		// Fallback: use the phone number itself.
		if pn.User != "" {
			m.SenderName = "+" + pn.User
		}
	}
}
