package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.mau.fi/whatsmeow/types"
)

type recordingGroupPhotoSetter struct {
	jid    types.JID
	avatar []byte
}

func (r *recordingGroupPhotoSetter) SetGroupPhoto(ctx context.Context, jid types.JID, avatar []byte) (string, error) {
	r.jid = jid
	r.avatar = append([]byte(nil), avatar...)
	return "picture-id", nil
}

func TestSetGroupPhotoFromFileReadsImageAndReturnsPictureID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "avatar.jpg")
	wantAvatar := []byte{0xff, 0xd8, 0xff, 0xdb}
	if err := os.WriteFile(path, wantAvatar, 0o600); err != nil {
		t.Fatal(err)
	}

	setter := &recordingGroupPhotoSetter{}
	jid, pictureID, err := setGroupPhotoFromFile(context.Background(), setter, "12345@g.us", path)

	if err != nil {
		t.Fatalf("setGroupPhotoFromFile: %v", err)
	}
	if pictureID != "picture-id" {
		t.Fatalf("pictureID = %q, want picture-id", pictureID)
	}
	if jid.String() != "12345@g.us" {
		t.Fatalf("jid = %s, want 12345@g.us", jid.String())
	}
	if setter.jid != jid {
		t.Fatalf("SetGroupPhoto jid = %s, want %s", setter.jid, jid)
	}
	if string(setter.avatar) != string(wantAvatar) {
		t.Fatalf("avatar bytes = %v, want %v", setter.avatar, wantAvatar)
	}
}

func TestSetGroupPhotoFromFileRequiresGroupJID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "avatar.jpg")
	if err := os.WriteFile(path, []byte{1}, 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, err := setGroupPhotoFromFile(context.Background(), &recordingGroupPhotoSetter{}, "12345@s.whatsapp.net", path)

	if err == nil {
		t.Fatal("expected error")
	}
}
