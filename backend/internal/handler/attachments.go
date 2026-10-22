package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"boodou/backend/internal/storage"
	"boodou/backend/internal/store"

	"github.com/gin-gonic/gin"
	_ "golang.org/x/image/webp"
)

const (
	MaxAttachmentBytes = 5 << 20
	orphanMaxAge       = 24 * time.Hour
)

// Extensions double as the allowlist: the type comes from the file's bytes,
// never from its name or the Content-Type the browser claims.
var imageExt = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type Attachments struct {
	store   *store.Store
	files   storage.Storage
	enabled bool
}

func NewAttachments(st *store.Store, files storage.Storage, vision bool) *Attachments {
	return &Attachments{store: st, files: files, enabled: vision}
}

func jsonError(c *gin.Context, status int, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"error": msg})
}

func (h *Attachments) Upload(c *gin.Context) {
	if !h.enabled {
		jsonError(c, http.StatusForbidden, "Model yang sedang dipakai tidak bisa membaca gambar, jadi lampiran dimatikan.")
		return
	}

	// Room for multipart headers on top of the file itself.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, MaxAttachmentBytes+64<<10)
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			jsonError(c, http.StatusRequestEntityTooLarge, "Gambar lebih dari 5 MB.")
			return
		}
		jsonError(c, http.StatusBadRequest, "Kirim gambar sebagai field multipart bernama file.")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, MaxAttachmentBytes+1))
	if err != nil {
		jsonError(c, http.StatusBadRequest, "Gambar gagal dibaca.")
		return
	}
	if len(data) > MaxAttachmentBytes {
		jsonError(c, http.StatusRequestEntityTooLarge, "Gambar lebih dari 5 MB.")
		return
	}
	if len(data) == 0 {
		jsonError(c, http.StatusBadRequest, "File kosong.")
		return
	}

	mime := http.DetectContentType(data)
	ext, ok := imageExt[mime]
	if !ok {
		jsonError(c, http.StatusUnsupportedMediaType, "File bukan gambar JPEG, PNG, atau WebP.")
		return
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || "image/"+format != mime || cfg.Width <= 0 || cfg.Height <= 0 {
		jsonError(c, http.StatusUnsupportedMediaType, "Gambar rusak atau tidak bisa dibaca.")
		return
	}

	sum := sha256.Sum256(data)
	a, err := h.store.CreateAttachment(c.Request.Context(), store.Attachment{
		MIME: mime, SizeBytes: len(data), Width: cfg.Width, Height: cfg.Height, SHA256: hex.EncodeToString(sum[:]),
	}, ext)
	if err != nil {
		log.Printf("attachments: simpan baris gagal: %v", err)
		jsonError(c, http.StatusInternalServerError, "Gambar gagal disimpan.")
		return
	}
	if err := h.files.Put(c.Request.Context(), a.StorageKey, bytes.NewReader(data)); err != nil {
		log.Printf("attachments: simpan file %s gagal: %v", a.StorageKey, err)
		_ = h.store.DeleteAttachment(context.WithoutCancel(c.Request.Context()), a.ID)
		jsonError(c, http.StatusInternalServerError, "Gambar gagal disimpan.")
		return
	}

	c.JSON(http.StatusCreated, a)
}

func (h *Attachments) Serve(c *gin.Context) {
	a, err := h.store.GetAttachment(c.Request.Context(), c.Param("id"))
	if errors.Is(err, store.ErrNotFound) {
		jsonError(c, http.StatusNotFound, "Gambar tidak ditemukan.")
		return
	}
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "Gambar gagal dimuat.")
		return
	}
	rc, err := h.files.Get(c.Request.Context(), a.StorageKey)
	if err != nil {
		log.Printf("attachments: file %s tidak bisa dibuka: %v", a.StorageKey, err)
		jsonError(c, http.StatusNotFound, "File gambar tidak ditemukan.")
		return
	}
	defer rc.Close()

	h2 := c.Writer.Header()
	h2.Set("Content-Type", a.MIME)
	h2.Set("Content-Length", strconv.Itoa(a.SizeBytes))
	// nosniff plus a locked-down CSP keep a crafted file from ever being
	// treated as HTML or script, even if opened directly.
	h2.Set("X-Content-Type-Options", "nosniff")
	h2.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h2.Set("Cache-Control", "private")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}

// CleanupOrphans removes uploads that were never sent with a message, such as
// images a user attached and then abandoned.
func CleanupOrphans(ctx context.Context, st *store.Store, files storage.Storage) (int, error) {
	keys, err := st.CleanupOrphans(ctx, orphanMaxAge)
	if err != nil {
		return 0, err
	}
	removeFiles(ctx, files, keys)
	return len(keys), nil
}

// removeFiles runs after the rows are gone; a file that fails to delete is
// logged by key only and left for manual cleanup rather than blocking.
func removeFiles(ctx context.Context, files storage.Storage, keys []string) {
	for _, k := range keys {
		if err := files.Delete(ctx, k); err != nil {
			log.Printf("storage: hapus %s gagal: %v", k, err)
		}
	}
}
