package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/app"
	"github.com/steipete/wacli/internal/config"
	"github.com/steipete/wacli/internal/ipc"
	"github.com/steipete/wacli/internal/out"
	"github.com/steipete/wacli/internal/wa"
	"go.mau.fi/whatsmeow/types"
)

type appHandle struct {
	app *app.App
	jid types.JID
}

type chatStateAction struct {
	use   string
	short string
	run   func(ctx context.Context, a *appHandle, jid string) error
}

func newChatStateCmd(flags *rootFlags, action chatStateAction) *cobra.Command {
	var jidStr string
	var noIPC bool
	cmd := &cobra.Command{
		Use:   action.use,
		Short: action.short,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}

			// Try IPC first if not disabled
			if !noIPC {
				storeDir := flags.storeDir
				if storeDir == "" {
					storeDir = config.DefaultStoreDir()
				}
				storeDir, _ = filepath.Abs(storeDir)

				client := ipc.NewClient(storeDir)
				if client.IsAvailable() {
					err := client.ChatState(jidStr, action.use, "")
					if err != nil {
						fmt.Fprintf(os.Stderr, "IPC %s failed (%v), trying direct mode...\n", action.use, err)
					} else {
						if flags.asJSON {
							return out.WriteJSON(os.Stdout, map[string]any{
								"jid":    jidStr,
								"action": action.use,
								"ok":     true,
								"via":    "ipc",
							})
						}
						fmt.Fprintln(os.Stdout, "OK (via daemon)")
						return nil
					}
				}
			}

			// Direct mode
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, true, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			if err := a.EnsureAuthed(); err != nil {
				return err
			}
			if err := a.Connect(ctx, false, nil); err != nil {
				return err
			}

			jid, err := wa.ParseUserOrJID(jidStr)
			if err != nil {
				return err
			}

			h := &appHandle{app: a, jid: jid}
			if err := action.run(ctx, h, jidStr); err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"jid":    jid.String(),
					"action": action.use,
					"ok":     true,
					"via":    "direct",
				})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "chat JID")
	cmd.Flags().BoolVar(&noIPC, "no-ipc", false, "skip IPC and use direct connection")
	return cmd
}

func newChatsMuteCmd(flags *rootFlags) *cobra.Command {
	var jidStr string
	var durStr string
	var noIPC bool
	cmd := &cobra.Command{
		Use:   "mute",
		Short: "Mute a chat",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(jidStr) == "" {
				return fmt.Errorf("--jid is required")
			}

			// Try IPC first if not disabled
			if !noIPC {
				storeDir := flags.storeDir
				if storeDir == "" {
					storeDir = config.DefaultStoreDir()
				}
				storeDir, _ = filepath.Abs(storeDir)

				client := ipc.NewClient(storeDir)
				if client.IsAvailable() {
					err := client.ChatState(jidStr, "mute", durStr)
					if err != nil {
						fmt.Fprintf(os.Stderr, "IPC mute failed (%v), trying direct mode...\n", err)
					} else {
						if flags.asJSON {
							return out.WriteJSON(os.Stdout, map[string]any{
								"jid":    jidStr,
								"action": "mute",
								"ok":     true,
								"via":    "ipc",
							})
						}
						fmt.Fprintln(os.Stdout, "OK (via daemon)")
						return nil
					}
				}
			}

			// Direct mode
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, true, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			if err := a.EnsureAuthed(); err != nil {
				return err
			}
			if err := a.Connect(ctx, false, nil); err != nil {
				return err
			}

			jid, err := wa.ParseUserOrJID(jidStr)
			if err != nil {
				return err
			}

			var dur time.Duration
			if strings.TrimSpace(durStr) != "" {
				dur, err = time.ParseDuration(durStr)
				if err != nil {
					return fmt.Errorf("invalid --duration: %w", err)
				}
			}

			if err := a.MuteChat(ctx, jid, true, dur); err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"jid":    jid.String(),
					"action": "mute",
					"ok":     true,
					"via":    "direct",
				})
			}
			fmt.Fprintln(os.Stdout, "OK")
			return nil
		},
	}
	cmd.Flags().StringVar(&jidStr, "jid", "", "chat JID")
	cmd.Flags().StringVar(&durStr, "duration", "", "mute duration (e.g. 8h, 24h, 168h); empty = forever")
	cmd.Flags().BoolVar(&noIPC, "no-ipc", false, "skip IPC and use direct connection")
	return cmd
}
