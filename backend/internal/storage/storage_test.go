package storage

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocal(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	key := "3f2b6c1e-0d7a-4a63-9f3e-2b1c0d9e8f7a.jpg"

	if err := s.Put(ctx, key, strings.NewReader("isi gambar")); err != nil {
		t.Fatal(err)
	}
	rc, err := s.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(rc)
	rc.Close()
	if string(got) != "isi gambar" {
		t.Fatalf("isi berbeda: %q", got)
	}

	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, key)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("file masih ada di disk setelah Delete")
	}
	if _, err := s.Get(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get setelah Delete: want ErrNotFound, got %v", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatalf("Delete kedua harus aman, got %v", err)
	}
}

func TestLocalRejectsUnsafeKeys(t *testing.T) {
	s, _ := NewLocal(t.TempDir())
	ctx := context.Background()
	for _, key := range []string{"", "../luar.jpg", `..\luar.jpg`, "sub/dir.jpg", "/abs.jpg", "C:x.jpg", ".hidden", "a b.jpg"} {
		if err := s.Put(ctx, key, strings.NewReader("x")); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Put(%q): want ErrInvalidKey, got %v", key, err)
		}
		if _, err := s.Get(ctx, key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Get(%q): want ErrInvalidKey, got %v", key, err)
		}
	}
}

func TestLocalLeavesNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	s, _ := NewLocal(dir)
	if err := s.Put(context.Background(), "a.png", strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "a.png" {
		t.Fatalf("isi folder: %v", entries)
	}
}
