package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

var ErrNoAPIKey = errors.New("LLM_API_KEY belum diisi")

// APIError carries a provider failure without the raw body, so callers can
// show Message to users without leaking provider internals.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string { return e.Message }

type Options struct {
	BaseURL string
	APIKey  string
	Model   string
	// QuotaReset is a human description of when the daily quota resets, e.g.
	// "tengah malam WIB". It differs per provider, so it comes from config.
	QuotaReset string
}

type Client struct {
	baseURL    string
	apiKey     string
	model      string
	quotaReset string
	http       *http.Client
}

func NewClient(opts Options) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// A whole-request timeout would cut off long streams, so only the wait for
	// the first response byte is bounded here; callers cap the total via ctx.
	transport.ResponseHeaderTimeout = 60 * time.Second

	return &Client{
		baseURL:    strings.TrimRight(opts.BaseURL, "/"),
		apiKey:     opts.APIKey,
		model:      opts.Model,
		quotaReset: opts.QuotaReset,
		http:       &http.Client{Transport: transport},
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// StreamChat calls onDelta for every content fragment until the provider sends
// [DONE], the stream ends, or ctx is cancelled.
func (c *Client) StreamChat(ctx context.Context, messages []Message, onDelta func(string) error) error {
	if c.apiKey == "" {
		return ErrNoAPIKey
	}

	body, err := json.Marshal(chatRequest{Model: c.model, Messages: messages, Stream: true})
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return classifyTransportError(ctx, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// The body is only inspected for keywords, never surfaced, because
		// some providers echo request headers (and so the key) in it.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return c.statusError(resp.StatusCode, resp.Header.Get("Retry-After"), string(snippet))
	}

	scanner := bufio.NewScanner(resp.Body)
	// Default 64 KiB line limit is too small for providers that send large
	// single-line chunks.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			return nil
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return &APIError{Status: http.StatusBadGateway, Message: "Provider mengirim error: " + chunk.Error.Message}
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			if err := onDelta(choice.Delta.Content); err != nil {
				return err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return classifyTransportError(ctx, err)
	}
	return nil
}

func (c *Client) statusError(status int, retryAfter, body string) *APIError {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &APIError{Status: status, Message: "API key ditolak provider: key salah atau sudah kedaluwarsa. Buat key baru, isi ke LLM_API_KEY di backend/.env, lalu jalankan ulang backend."}
	case http.StatusNotFound:
		return &APIError{Status: status, Message: "Endpoint atau model tidak ditemukan. Periksa LLM_BASE_URL dan LLM_MODEL."}
	case http.StatusTooManyRequests:
		return &APIError{Status: status, Message: c.rateLimitMessage(retryAfter, body)}
	default:
		if status >= 500 {
			return &APIError{Status: http.StatusBadGateway, Message: fmt.Sprintf("Provider sedang bermasalah (status %d). Coba lagi nanti.", status)}
		}
		return &APIError{Status: http.StatusBadGateway, Message: fmt.Sprintf("Provider menolak permintaan (status %d).", status)}
	}
}

// A 429 can mean a short per-minute burst or a spent daily quota, and the
// user needs to know which one to decide whether to wait or stop. Providers
// differ in how they say it, so Retry-After and body keywords are both used.
func (c *Client) rateLimitMessage(retryAfter, body string) string {
	if secs, err := strconv.Atoi(strings.TrimSpace(retryAfter)); err == nil && secs > 0 && secs <= 300 {
		return fmt.Sprintf("Terlalu banyak pesan dalam waktu singkat. Coba lagi dalam %d detik.", secs)
	}

	lower := strings.ToLower(body)
	daily := containsAny(lower, "day", "daily", "harian", "quota", "kuota")
	burst := containsAny(lower, "minute", "per menit", "rpm")

	reset := "Coba lagi nanti."
	if c.quotaReset != "" {
		reset = "Kuota direset " + c.quotaReset + "."
	}

	switch {
	case burst && !daily:
		return "Terlalu banyak pesan dalam waktu singkat. Tunggu sekitar satu menit lalu coba lagi."
	case daily:
		return "Kuota harian provider sudah habis. " + reset
	default:
		return "Batas pemakaian provider tercapai. Kalau baru saja mengirim banyak pesan, tunggu sekitar satu menit. Kalau masih gagal, kuota harian sudah habis. " + reset
	}
}

func containsAny(s string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

func classifyTransportError(ctx context.Context, err error) error {
	// Client disconnects surface as context.Canceled; pass them through so the
	// handler can stop quietly instead of reporting to a closed connection.
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &netErr) && netErr.Timeout()) {
		return &APIError{Status: http.StatusGatewayTimeout, Message: "Provider tidak merespons tepat waktu. Coba lagi."}
	}
	return &APIError{Status: http.StatusBadGateway, Message: "Tidak bisa terhubung ke provider. Periksa LLM_BASE_URL dan koneksi internet."}
}
