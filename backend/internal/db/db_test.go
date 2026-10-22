package db_test

import (
	"context"
	"strings"
	"testing"

	"boodou/backend/internal/db"
	"boodou/backend/internal/testdb"
)

func TestMigrateIsIdempotent(t *testing.T) {
	pool := testdb.Open(t)
	applied, err := db.Migrate(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 0 {
		t.Fatalf("migrasi kedua seharusnya tidak menjalankan apa pun, jalan: %v", applied)
	}
}

func TestSchemaConstraints(t *testing.T) {
	pool := testdb.Open(t)
	ctx := context.Background()

	var conv string
	if err := pool.QueryRow(ctx, `INSERT INTO conversations (title) VALUES ('x') RETURNING id`).Scan(&conv); err != nil {
		t.Fatal(err)
	}

	// The persona lives only in code; the schema must refuse to store it.
	if _, err := pool.Exec(ctx, `INSERT INTO messages (conversation_id, role, content) VALUES ($1, 'system', 'x')`, conv); err == nil {
		t.Fatal("role system seharusnya ditolak")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO messages (conversation_id, role, content, status) VALUES ($1, 'user', 'x', 'pending')`, conv); err == nil {
		t.Fatal("status di luar complete/stopped/error seharusnya ditolak")
	}

	var msg string
	if err := pool.QueryRow(ctx, `INSERT INTO messages (conversation_id, role, content) VALUES ($1, 'user', 'halo') RETURNING id`, conv).Scan(&msg); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO attachments (message_id, mime, size_bytes, width, height, sha256, storage_key)
		VALUES ($1, 'image/jpeg', 10, 1, 1, 'h', 'k1')`, msg); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO attachments (mime, size_bytes, width, height, sha256, storage_key)
		VALUES ('image/gif', 10, 1, 1, 'h', 'k2')`); err == nil || !strings.Contains(err.Error(), "check") {
		t.Fatalf("mime gif seharusnya ditolak, err=%v", err)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, conv); err != nil {
		t.Fatal(err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM messages) + (SELECT count(*) FROM attachments)`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("hapus percakapan harus menghapus pesan dan lampiran (cascade), sisa %d", left)
	}
}
