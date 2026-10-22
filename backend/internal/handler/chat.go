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

	"boodou/backend/internal/llm"
	"boodou/backend/internal/persona"

	"github.com/gin-gonic/gin"
)

const (
	maxBodyBytes  = 1 << 20
	streamTimeout = 3 * time.Minute
)

type chatRequest struct {
	Messages []llm.Message `json:"messages"`
}

type Chat struct {
	client *llm.Client
	model  string
	now    func() time.Time
}

func NewChat(client *llm.Client, model string) *Chat {
	return &Chat{client: client, model: model, now: time.Now}
}

// Every outcome, including validation errors, is delivered as SSE so the
// frontend needs only one parser. Headers are held back until the first token
// so failures before that can still carry a meaningful HTTP status.
func (h *Chat) Handle(c *gin.Context) {
	stream := &sseWriter{c: c}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	var req chatRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		stream.fail(http.StatusBadRequest, "Format permintaan tidak valid.")
		return
	}
	// net/http only notices a client disconnect (and cancels the request
	// context) after the body has been read to EOF; Decode may stop short.
	_, _ = io.Copy(io.Discard, c.Request.Body)
	messages := persona.Apply(h.model, h.now(), req.Messages)
	// Validate only what the client contributed; the persona is always valid.
	if msg := validate(messages[1:]); msg != "" {
		stream.fail(http.StatusBadRequest, msg)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), streamTimeout)
	defer cancel()

	err := h.client.StreamChat(ctx, messages, stream.token)
	if err == nil {
		stream.done()
		return
	}

	var apiErr *llm.APIError
	switch {
	case errors.Is(err, context.Canceled) || c.Request.Context().Err() != nil:
		// Client went away; write errors land here too, and there is nobody
		// left to report them to.
	case errors.Is(err, llm.ErrNoAPIKey):
		stream.fail(http.StatusServiceUnavailable, "Server belum dikonfigurasi: LLM_API_KEY masih kosong di backend/.env.")
	case errors.As(err, &apiErr):
		log.Printf("chat: provider error status=%d: %s", apiErr.Status, apiErr.Message)
		stream.fail(apiErr.Status, apiErr.Message)
	default:
		log.Printf("chat: unexpected error: %v", err)
		stream.fail(http.StatusInternalServerError, "Terjadi kesalahan di server. Coba lagi.")
	}
}

func validate(messages []llm.Message) string {
	if len(messages) == 0 {
		return "Daftar pesan tidak boleh kosong."
	}
	for i, m := range messages {
		switch m.Role {
		case "user", "assistant":
		default:
			return fmt.Sprintf("Pesan ke-%d punya role yang tidak dikenal.", i+1)
		}
		if strings.TrimSpace(m.Content) == "" {
			return fmt.Sprintf("Pesan ke-%d kosong.", i+1)
		}
	}
	if messages[len(messages)-1].Role != "user" {
		return "Pesan terakhir harus dari user."
	}
	return ""
}

type sseWriter struct {
	c       *gin.Context
	started bool
}

func (s *sseWriter) begin(status int) {
	if s.started {
		return
	}
	s.started = true
	h := s.c.Writer.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// Stops reverse proxies like nginx from buffering the whole stream.
	h.Set("X-Accel-Buffering", "no")
	s.c.Status(status)
}

func (s *sseWriter) send(event string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.c.Writer, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	s.c.Writer.Flush()
	return nil
}

func (s *sseWriter) token(content string) error {
	s.begin(http.StatusOK)
	return s.send("token", gin.H{"content": content})
}

func (s *sseWriter) done() {
	s.begin(http.StatusOK)
	_ = s.send("done", gin.H{})
}

func (s *sseWriter) fail(status int, message string) {
	s.begin(status)
	_ = s.send("error", gin.H{"message": message})
}
