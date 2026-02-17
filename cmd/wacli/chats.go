package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/out"
	"github.com/steipete/wacli/internal/store"
)

func newChatsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "chats",
		Short: "List and manage chats",
	}
	cmd.AddCommand(newChatsListCmd(flags))
	cmd.AddCommand(newChatsShowCmd(flags))
	cmd.AddCommand(newChatStateCmd(flags, chatStateAction{
		use: "archive", short: "Archive a chat",
		run: func(ctx context.Context, a *appHandle, jid string) error { return a.app.ArchiveChat(ctx, a.jid, true) },
	}))
	cmd.AddCommand(newChatStateCmd(flags, chatStateAction{
		use: "unarchive", short: "Unarchive a chat",
		run: func(ctx context.Context, a *appHandle, jid string) error { return a.app.ArchiveChat(ctx, a.jid, false) },
	}))
	cmd.AddCommand(newChatStateCmd(flags, chatStateAction{
		use: "pin", short: "Pin a chat",
		run: func(ctx context.Context, a *appHandle, jid string) error { return a.app.PinChat(ctx, a.jid, true) },
	}))
	cmd.AddCommand(newChatStateCmd(flags, chatStateAction{
		use: "unpin", short: "Unpin a chat",
		run: func(ctx context.Context, a *appHandle, jid string) error { return a.app.PinChat(ctx, a.jid, false) },
	}))
	cmd.AddCommand(newChatsMuteCmd(flags))
	cmd.AddCommand(newChatStateCmd(flags, chatStateAction{
		use: "unmute", short: "Unmute a chat",
		run: func(ctx context.Context, a *appHandle, jid string) error { return a.app.MuteChat(ctx, a.jid, false, 0) },
	}))
	cmd.AddCommand(newChatStateCmd(flags, chatStateAction{
		use: "mark-read", short: "Mark a chat as read",
		run: func(ctx context.Context, a *appHandle, jid string) error { return a.app.MarkChatRead(ctx, a.jid, true) },
	}))
	cmd.AddCommand(newChatStateCmd(flags, chatStateAction{
		use: "mark-unread", short: "Mark a chat as unread",
		run: func(ctx context.Context, a *appHandle, jid string) error { return a.app.MarkChatRead(ctx, a.jid, false) },
	}))
	return cmd
}

func newChatsListCmd(flags *rootFlags) *cobra.Command {
	var query string
	var limit int
	var archived, noArchived bool
	var pinned, noPinned bool
	var muted, noMuted bool
	var unread, noUnread bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List chats",
		RunE: func(cmd *cobra.Command, args []string) error {
			if archived && noArchived {
				return fmt.Errorf("--archived and --no-archived are mutually exclusive")
			}
			if pinned && noPinned {
				return fmt.Errorf("--pinned and --no-pinned are mutually exclusive")
			}
			if muted && noMuted {
				return fmt.Errorf("--muted and --no-muted are mutually exclusive")
			}
			if unread && noUnread {
				return fmt.Errorf("--unread and --no-unread are mutually exclusive")
			}

			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			f := store.ChatListFilter{Query: query, Limit: limit}
			f.Archived = boolFilter(archived, noArchived)
			f.Pinned = boolFilter(pinned, noPinned)
			f.Muted = boolFilter(muted, noMuted)
			f.Unread = boolFilter(unread, noUnread)

			chats, err := a.DB().ListChats(f)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, chats)
			}

			w := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
			fmt.Fprintln(w, "KIND\tNAME\tJID\tLAST\tFLAGS")
			for _, c := range chats {
				name := c.Name
				if name == "" {
					name = c.JID
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", c.Kind, truncate(name, 28), c.JID, c.LastMessageTS.Local().Format("2006-01-02 15:04:05"), chatFlagsString(c))
			}
			_ = w.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "search query")
	cmd.Flags().IntVar(&limit, "limit", 50, "limit")
	cmd.Flags().BoolVar(&archived, "archived", false, "show only archived chats")
	cmd.Flags().BoolVar(&noArchived, "no-archived", false, "exclude archived chats")
	cmd.Flags().BoolVar(&pinned, "pinned", false, "show only pinned chats")
	cmd.Flags().BoolVar(&noPinned, "no-pinned", false, "exclude pinned chats")
	cmd.Flags().BoolVar(&muted, "muted", false, "show only muted chats")
	cmd.Flags().BoolVar(&noMuted, "no-muted", false, "exclude muted chats")
	cmd.Flags().BoolVar(&unread, "unread", false, "show only unread chats")
	cmd.Flags().BoolVar(&noUnread, "no-unread", false, "exclude unread chats")
	return cmd
}

func newChatsShowCmd(flags *rootFlags) *cobra.Command {
	var jid string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show one chat",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jid == "" {
				return fmt.Errorf("--jid is required")
			}
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, false, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			c, err := a.DB().GetChat(jid)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, c)
			}
			fmt.Fprintf(os.Stdout, "JID: %s\nKind: %s\nName: %s\nLast: %s\nArchived: %t\nPinned: %t\nMuted: %t\nUnread: %t\n",
				c.JID, c.Kind, c.Name, c.LastMessageTS.Local().Format(time.RFC3339),
				c.Archived, c.Pinned, c.Muted(), c.Unread)
			return nil
		},
	}
	cmd.Flags().StringVar(&jid, "jid", "", "chat JID")
	return cmd
}

func boolFilter(pos, neg bool) *bool {
	if pos {
		v := true
		return &v
	}
	if neg {
		v := false
		return &v
	}
	return nil
}

func chatFlagsString(c store.Chat) string {
	var flags []string
	if c.Pinned {
		flags = append(flags, "pinned")
	}
	if c.Archived {
		flags = append(flags, "archived")
	}
	if c.Muted() {
		flags = append(flags, "muted")
	}
	if c.Unread {
		flags = append(flags, "unread")
	}
	return strings.Join(flags, ",")
}
