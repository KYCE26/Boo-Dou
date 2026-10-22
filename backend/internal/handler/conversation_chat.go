package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"boodou/backend/internal/llm"
	"boodou/backend/internal/persona"
	"boodou/backend/internal/storage"
	"boodou/backend/internal/store"

	"github.com/gin-gonic/gin"
)

const (
	MaxImagesPerMessage = 4
	maxTextRunes        = 32000
	// Only the latest image-bearing user turns are resent; older images cost
	// about a thousand tokens each and rarely matter to the next answer.
	imageHistoryWindow = 2
	olderImageNote     = "[gambar sebelumnya tidak dikirim ulang]"
	missingImageNote   = "[gambar tidak tersedia]"
	saveTimeout        = 10 * time.Second
)

type ConversationChat struct {
	store  *store.Store
	files  storage.Storage
	client *llm.Client
	model  string
	vision bool
	now    func() time.Time
}

func NewConversationChat(st *store.Store, files storage.Storage, client *llm.Client, model string, vision bool) *ConversationChat {
	return &ConversationChat{store: st, files: files, client: client, model: model, vision: vision, now: time.Now}
}

type conversationChatBody struct {
	Text          string   `json:"text"`
	AttachmentIDs []string `json:"attachment_ids"`
	// Retry regenerates the answer to the last user message after a failure,
	// without saving that message again.
	Retry bool `json:"retry"`
}

func (h *ConversationChat) Handle(c *gin.Context) {
	stream := &sseWriter{c: c}
	convID := c.Param("id")

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	var body conversationChatBody
	if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
		stream.fail(http.StatusBadRequest, "Format permintaan tidak valid.")
		return
	}
	// Reading to EOF is what lets net/http notice a client disconnect.
	_, _ = io.Copy(io.Discard, c.Request.Body)

	text := strings.TrimSpace(body.Text)
	ids := dedupe(body.AttachmentIDs)
	if msg := h.validate(text, ids, body.Retry); msg != "" {
		stream.fail(http.StatusBadRequest, msg)
		return
	}
	// Checked before saving so an unconfigured server never stores a
	// question it cannot answer.
	if !h.client.Configured() {
		stream.fail(http.StatusServiceUnavailable, "Server belum dikonfigurasi: LLM_API_KEY masih kosong di backend/.env.")
		return
	}

	ctx := c.Request.Context()
	if body.Retry {
		ok, err := h.store.PrepareRetry(ctx, convID)
		if h.failOnStoreError(stream, err) {
			return
		}
		if !ok {
			stream.fail(http.StatusBadRequest, "Tidak ada pesan yang bisa dikirim ulang.")
			return
		}
	} else {
		_, err := h.store.AddUserMessage(ctx, convID, text, ids)
		if h.failOnStoreError(stream, err) {
			return
		}
	}

	history, err := h.store.Messages(ctx, convID)
	if err != nil {
		log.Printf("chat: muat riwayat: %v", err)
		stream.fail(http.StatusInternalServerError, "Riwayat percakapan gagal dimuat.")
		return
	}
	messages, sentImages := h.buildHistory(ctx, history)

	streamCtx, cancel := context.WithTimeout(ctx, streamTimeout)
	defer cancel()

	var answer strings.Builder
	err = h.client.StreamChat(streamCtx, persona.Apply(h.model, h.now(), messages), func(delta string) error {
		answer.WriteString(delta)
		return stream.token(delta)
	})

	status := store.StatusComplete
	disconnected := ctx.Err() != nil
	switch {
	case err == nil:
	case errors.Is(err, context.Canceled) || disconnected:
		status = store.StatusStopped
	default:
		status = store.StatusError
	}

	saved, saveErr := h.saveAnswer(c, convID, answer.String(), status)
	if disconnected {
		return
	}
	if err != nil {
		h.failStream(stream, err, sentImages)
		return
	}
	if saveErr != nil {
		stream.fail(http.StatusInternalServerError, "Jawaban diterima tetapi gagal disimpan.")
		return
	}
	stream.finish(gin.H{"message_id": saved.ID})
}

func (h *ConversationChat) validate(text string, ids []string, retry bool) string {
	if retry {
		if text != "" || len(ids) > 0 {
			return "Kirim ulang tidak boleh membawa pesan atau gambar baru."
		}
		return ""
	}
	if text == "" && len(ids) == 0 {
		return "Pesan tidak boleh kosong."
	}
	if utf8.RuneCountInString(text) > maxTextRunes {
		return fmt.Sprintf("Pesan terlalu panjang, maksimal %d karakter.", maxTextRunes)
	}
	if len(ids) > 0 && !h.vision {
		return "Model yang sedang dipakai tidak bisa membaca gambar. Kirim pesan tanpa gambar."
	}
	if len(ids) > MaxImagesPerMessage {
		return fmt.Sprintf("Maksimal %d gambar per pesan.", MaxImagesPerMessage)
	}
	return ""
}

func (h *ConversationChat) failOnStoreError(stream *sseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, store.ErrNotFound):
		stream.fail(http.StatusNotFound, "Percakapan tidak ditemukan.")
	case errors.Is(err, store.ErrAttachmentUnavailable):
		stream.fail(http.StatusBadRequest, "Ada gambar yang tidak ditemukan atau sudah dipakai di pesan lain. Lampirkan ulang gambarnya.")
	default:
		log.Printf("chat: simpan pesan: %v", err)
		stream.fail(http.StatusInternalServerError, "Pesan gagal disimpan.")
	}
	return true
}

// saveAnswer outlives the request: a stopped answer is saved precisely when
// the client has gone away. Empty stopped answers have nothing worth keeping.
func (h *ConversationChat) saveAnswer(c *gin.Context, convID, content, status string) (store.Message, error) {
	if status == store.StatusStopped && content == "" {
		return store.Message{}, nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), saveTimeout)
	defer cancel()
	m, err := h.store.AddAssistantMessage(ctx, convID, content, status, h.model)
	if err != nil {
		log.Printf("chat: simpan jawaban (%s): %v", status, err)
	}
	return m, err
}

func (h *ConversationChat) failStream(stream *sseWriter, err error, sentImages bool) {
	var apiErr *llm.APIError
	switch {
	case errors.Is(err, llm.ErrNoAPIKey):
		stream.fail(http.StatusServiceUnavailable, "Server belum dikonfigurasi: LLM_API_KEY masih kosong di backend/.env.")
	case errors.As(err, &apiErr):
		log.Printf("chat: provider error status=%d provider=%d: %s", apiErr.Status, apiErr.ProviderStatus, apiErr.Message)
		if sentImages && isImageRejection(apiErr.ProviderStatus) {
			stream.fail(http.StatusBadGateway, fmt.Sprintf("Provider menolak gambar yang dikirim (status %d). Coba gambar lain, atau kirim tanpa gambar.", apiErr.ProviderStatus))
			return
		}
		stream.fail(apiErr.Status, apiErr.Message)
	default:
		log.Printf("chat: unexpected error: %v", err)
		stream.fail(http.StatusInternalServerError, "Terjadi kesalahan di server. Coba lagi.")
	}
}

func isImageRejection(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnsupportedMediaType, http.StatusUnprocessableEntity:
		return true
	}
	return false
}

// buildHistory turns stored messages into provider messages. Failed answers
// are skipped, and images are attached only for the last few user turns that
// have them; older ones are replaced by a note so the model knows they existed.
func (h *ConversationChat) buildHistory(ctx context.Context, history []store.Message) ([]llm.Message, bool) {
	withImages := make(map[int]bool)
	for i := len(history) - 1; i >= 0 && len(withImages) < imageHistoryWindow; i-- {
		if history[i].Role == store.RoleUser && len(history[i].Attachments) > 0 {
			withImages[i] = true
		}
	}

	out := make([]llm.Message, 0, len(history))
	sent := false
	for i, m := range history {
		if m.Role == store.RoleAssistant && (m.Status == store.StatusError || m.Content == "") {
			continue
		}
		msg := llm.Message{Role: m.Role, Content: m.Content}
		if len(m.Attachments) > 0 {
			if withImages[i] {
				msg.Images, msg.Content = h.loadImages(ctx, m)
				sent = sent || len(msg.Images) > 0
			} else {
				msg.Content = joinNote(m.Content, olderImageNote)
			}
		}
		out = append(out, msg)
	}
	return out, sent
}

func (h *ConversationChat) loadImages(ctx context.Context, m store.Message) ([]llm.Image, string) {
	content := m.Content
	images := make([]llm.Image, 0, len(m.Attachments))
	for _, a := range m.Attachments {
		data, err := h.readFile(ctx, a.StorageKey)
		if err != nil {
			log.Printf("chat: baca lampiran %s: %v", a.StorageKey, err)
			content = joinNote(content, missingImageNote)
			continue
		}
		images = append(images, llm.Image{MIME: a.MIME, Data: data})
	}
	return images, content
}

func (h *ConversationChat) readFile(ctx context.Context, key string) ([]byte, error) {
	rc, err := h.files.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, MaxAttachmentBytes+1))
}

func joinNote(content, note string) string {
	if content == "" {
		return note
	}
	return content + "\n\n" + note
}

func dedupe(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
