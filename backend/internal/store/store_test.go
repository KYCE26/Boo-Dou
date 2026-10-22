package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"boodou/backend/internal/store"
	"boodou/backend/internal/testdb"
)

func attach(t *testing.T, s *store.Store) store.Attachment {
	t.Helper()
	a, err := s.CreateAttachment(context.Background(), store.Attachment{MIME: "image/jpeg", SizeBytes: 10, Width: 2, Height: 3, SHA256: "h"}, ".jpg")
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestConversationLifecycle(t *testing.T) {
	s := store.New(testdb.Open(t))
	ctx := context.Background()

	c, err := s.CreateConversation(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	a1, a2 := attach(t, s), attach(t, s)
	if a1.StorageKey != a1.ID+".jpg" {
		t.Fatalf("storage key harus dari id, got %q", a1.StorageKey)
	}

	m, err := s.AddUserMessage(ctx, c.ID, "Apa isi kedua gambar ini?", []string{a2.ID, a1.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Attachments) != 2 || m.Attachments[0].ID != a2.ID {
		t.Fatalf("lampiran harus terikat sesuai urutan kiriman: %+v", m.Attachments)
	}
	if _, err := s.AddAssistantMessage(ctx, c.ID, "Jawaban terpotong", store.StatusStopped, "model-x"); err != nil {
		t.Fatal(err)
	}

	got, msgs, err := s.GetConversation(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Apa isi kedua gambar ini?" {
		t.Fatalf("judul dari pesan pertama, got %q", got.Title)
	}
	if len(msgs) != 2 || msgs[0].Role != "user" || len(msgs[0].Attachments) != 2 || msgs[1].Status != "stopped" || msgs[1].Model != "model-x" {
		t.Fatalf("isi percakapan: %+v", msgs)
	}
	// a1 was uploaded first but attached second; history must keep the user's order.
	if msgs[0].Attachments[0].ID != a2.ID || msgs[0].Attachments[1].ID != a1.ID {
		t.Fatalf("urutan lampiran di riwayat harus mengikuti urutan kiriman, bukan waktu upload")
	}

	// A second user message must not overwrite the title.
	if _, err := s.AddUserMessage(ctx, c.ID, "pertanyaan lain", nil); err != nil {
		t.Fatal(err)
	}
	if got, _, _ = s.GetConversation(ctx, c.ID); got.Title != "Apa isi kedua gambar ini?" {
		t.Fatalf("judul berubah: %q", got.Title)
	}

	renamed, err := s.RenameConversation(ctx, c.ID, "Judul baru")
	if err != nil || renamed.Title != "Judul baru" {
		t.Fatalf("rename: %+v %v", renamed, err)
	}

	keys, err := s.DeleteConversation(ctx, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("hapus harus mengembalikan key file lampiran, got %v", keys)
	}
	if _, _, err := s.GetConversation(ctx, c.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("percakapan masih ada: %v", err)
	}
	if _, err := s.GetAttachment(ctx, a1.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("lampiran harus ikut terhapus (cascade)")
	}
}

func TestAttachmentCannotBeReused(t *testing.T) {
	s := store.New(testdb.Open(t))
	ctx := context.Background()
	c, _ := s.CreateConversation(ctx, "")
	a := attach(t, s)

	if _, err := s.AddUserMessage(ctx, c.ID, "satu", []string{a.ID}); err != nil {
		t.Fatal(err)
	}
	_, err := s.AddUserMessage(ctx, c.ID, "dua", []string{a.ID})
	if !errors.Is(err, store.ErrAttachmentUnavailable) {
		t.Fatalf("lampiran terpakai harus ditolak, got %v", err)
	}
	// The failed claim must roll back the whole message.
	_, msgs, _ := s.GetConversation(ctx, c.ID)
	if len(msgs) != 1 {
		t.Fatalf("pesan gagal tidak boleh tersimpan setengah, ada %d pesan", len(msgs))
	}

	if _, err := s.AddUserMessage(ctx, c.ID, "x", []string{"bukan-uuid"}); !errors.Is(err, store.ErrAttachmentUnavailable) {
		t.Fatalf("id tidak valid: %v", err)
	}
	if _, err := s.AddUserMessage(ctx, "bukan-uuid", "x", nil); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("percakapan tidak valid: %v", err)
	}
}

func TestListOrderAndOrphanCleanup(t *testing.T) {
	pool := testdb.Open(t)
	s := store.New(pool)
	ctx := context.Background()

	old, _ := s.CreateConversation(ctx, "lama")
	recent, _ := s.CreateConversation(ctx, "baru")
	if _, err := s.AddUserMessage(ctx, old.ID, "naik ke atas", nil); err != nil {
		t.Fatal(err)
	}
	list, err := s.ListConversations(ctx)
	if err != nil || len(list) != 2 || list[0].ID != old.ID || list[1].ID != recent.ID {
		t.Fatalf("urutan updated_at terbaru dulu: %+v %v", list, err)
	}

	fresh := attach(t, s)
	stale := attach(t, s)
	if _, err := pool.Exec(ctx, `UPDATE attachments SET created_at = now() - interval '25 hours' WHERE id = $1`, stale.ID); err != nil {
		t.Fatal(err)
	}
	keys, err := s.CleanupOrphans(ctx, 24*time.Hour)
	if err != nil || len(keys) != 1 || keys[0] != stale.StorageKey {
		t.Fatalf("hanya lampiran yatim > 24 jam yang dihapus: %v %v", keys, err)
	}
	if _, err := s.GetAttachment(ctx, fresh.ID); err != nil {
		t.Fatal("lampiran baru tidak boleh ikut terhapus")
	}
}

func TestPrepareRetry(t *testing.T) {
	s := store.New(testdb.Open(t))
	ctx := context.Background()
	c, _ := s.CreateConversation(ctx, "")

	if ok, _ := s.PrepareRetry(ctx, c.ID); ok {
		t.Fatal("percakapan kosong tidak bisa di-retry")
	}
	_, _ = s.AddUserMessage(ctx, c.ID, "halo", nil)
	_, _ = s.AddAssistantMessage(ctx, c.ID, "", store.StatusError, "m")
	ok, err := s.PrepareRetry(ctx, c.ID)
	if err != nil || !ok {
		t.Fatalf("retry setelah error: %v %v", ok, err)
	}
	_, msgs, _ := s.GetConversation(ctx, c.ID)
	if len(msgs) != 1 || msgs[0].Role != "user" {
		t.Fatalf("jawaban error harus dibuang: %+v", msgs)
	}

	_, _ = s.AddAssistantMessage(ctx, c.ID, "jawaban", store.StatusComplete, "m")
	if ok, _ := s.PrepareRetry(ctx, c.ID); ok {
		t.Fatal("jawaban yang sudah selesai tidak boleh di-retry")
	}
}
