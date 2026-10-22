package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	maxBodyBytes  = 1 << 20
	streamTimeout = 3 * time.Minute
)

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

// finish ends a successful stream; the payload tells the client what was saved.
func (s *sseWriter) finish(payload any) {
	s.begin(http.StatusOK)
	_ = s.send("done", payload)
}

func (s *sseWriter) fail(status int, message string) {
	s.begin(status)
	_ = s.send("error", gin.H{"message": message})
}
