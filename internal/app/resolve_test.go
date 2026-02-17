package app

import (
	"context"
	"testing"

	"github.com/steipete/wacli/internal/store"
	"go.mau.fi/whatsmeow/types"
)

func TestResolveSenderNames_LIDWithContact(t *testing.T) {
	a := newTestApp(t)
	f := newFakeWA()
	a.wa = f

	lid := types.JID{User: "248317879549994", Server: types.HiddenUserServer}
	pn := types.JID{User: "4915112345678", Server: types.DefaultUserServer}
	f.lidMap[lid] = pn
	f.contacts[pn] = types.ContactInfo{Found: true, FullName: "Max Mustermann"}

	msgs := []store.Message{
		{SenderJID: lid.String(), SenderName: ""},
	}
	a.ResolveSenderNames(context.Background(), msgs)

	if msgs[0].SenderName != "Max Mustermann" {
		t.Fatalf("expected SenderName='Max Mustermann', got %q", msgs[0].SenderName)
	}
}

func TestResolveSenderNames_LIDWithoutContact(t *testing.T) {
	a := newTestApp(t)
	f := newFakeWA()
	a.wa = f

	lid := types.JID{User: "248317879549994", Server: types.HiddenUserServer}
	pn := types.JID{User: "4915112345678", Server: types.DefaultUserServer}
	f.lidMap[lid] = pn
	// No contact entry for pn.

	msgs := []store.Message{
		{SenderJID: lid.String(), SenderName: ""},
	}
	a.ResolveSenderNames(context.Background(), msgs)

	if msgs[0].SenderName != "+4915112345678" {
		t.Fatalf("expected SenderName='+4915112345678', got %q", msgs[0].SenderName)
	}
}

func TestResolveSenderNames_LIDNotInMap(t *testing.T) {
	a := newTestApp(t)
	f := newFakeWA()
	a.wa = f

	lid := types.JID{User: "999999999999999", Server: types.HiddenUserServer}

	msgs := []store.Message{
		{SenderJID: lid.String(), SenderName: ""},
	}
	a.ResolveSenderNames(context.Background(), msgs)

	if msgs[0].SenderName != "" {
		t.Fatalf("expected SenderName to remain empty, got %q", msgs[0].SenderName)
	}
}

func TestResolveSenderNames_NormalJIDUnchanged(t *testing.T) {
	a := newTestApp(t)
	f := newFakeWA()
	a.wa = f

	msgs := []store.Message{
		{SenderJID: "4915112345678@s.whatsapp.net", SenderName: ""},
	}
	a.ResolveSenderNames(context.Background(), msgs)

	if msgs[0].SenderName != "" {
		t.Fatalf("expected SenderName to remain empty for non-LID, got %q", msgs[0].SenderName)
	}
}

func TestResolveSenderNames_AlreadyResolved(t *testing.T) {
	a := newTestApp(t)
	f := newFakeWA()
	a.wa = f

	lid := types.JID{User: "248317879549994", Server: types.HiddenUserServer}
	pn := types.JID{User: "4915112345678", Server: types.DefaultUserServer}
	f.lidMap[lid] = pn
	f.contacts[pn] = types.ContactInfo{Found: true, FullName: "Max Mustermann"}

	msgs := []store.Message{
		{SenderJID: lid.String(), SenderName: "Already Known"},
	}
	a.ResolveSenderNames(context.Background(), msgs)

	if msgs[0].SenderName != "Already Known" {
		t.Fatalf("expected SenderName to remain 'Already Known', got %q", msgs[0].SenderName)
	}
}

func TestResolveSenderNames_FromMeSkipped(t *testing.T) {
	a := newTestApp(t)
	f := newFakeWA()
	a.wa = f

	msgs := []store.Message{
		{SenderJID: "248317879549994@lid", FromMe: true, SenderName: ""},
	}
	a.ResolveSenderNames(context.Background(), msgs)

	if msgs[0].SenderName != "" {
		t.Fatalf("expected SenderName to remain empty for FromMe, got %q", msgs[0].SenderName)
	}
}

func TestResolveSenderNames_EmptySlice(t *testing.T) {
	a := newTestApp(t)
	// No WA client set — should not panic.
	a.ResolveSenderNames(context.Background(), nil)
	a.ResolveSenderNames(context.Background(), []store.Message{})
}
