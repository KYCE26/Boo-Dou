// Package storage keeps attachment bytes behind an interface so the disk
// backend can later be swapped for an object store such as Cloudflare R2
// without touching handlers.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

var (
	ErrNotFound   = errors.New("storage: file tidak ditemukan")
	ErrInvalidKey = errors.New("storage: key tidak valid")
)

type Storage interface {
	Put(ctx context.Context, key string, r io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// Keys become file names, so anything beyond a flat, plain name is refused;
// this rules out path traversal even if a key ever comes from input.
var validKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*(\.[A-Za-z0-9]+)?$`)

func checkKey(key string) error {
	if len(key) > 128 || !validKey.MatchString(key) {
		return ErrInvalidKey
	}
	return nil
}

type Local struct {
	dir string
}

func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("buat folder upload: %w", err)
	}
	return &Local{dir: dir}, nil
}

// Put writes to a temp file and renames it, so a crash or a failed upload
// never leaves a truncated file under the final name.
func (l *Local) Put(_ context.Context, key string, r io.Reader) error {
	if err := checkKey(key); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(l.dir, ".upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), filepath.Join(l.dir, key))
}

func (l *Local) Get(_ context.Context, key string) (io.ReadCloser, error) {
	if err := checkKey(key); err != nil {
		return nil, err
	}
	f, err := os.Open(filepath.Join(l.dir, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

// Delete treats a missing file as success so cleanup can be retried safely.
func (l *Local) Delete(_ context.Context, key string) error {
	if err := checkKey(key); err != nil {
		return err
	}
	err := os.Remove(filepath.Join(l.dir, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
