package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestStreamChatCollectsDeltas(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer k" {
			t.Errorf("unexpected request %s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, part := range []string{"Ha", "lo"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", part)
		}
		fmt.Fprint(w, ": keep-alive\n\ndata: [DONE]\n\n")
	}))
	defer srv.Close()

	var got strings.Builder
	err := NewClient(Options{BaseURL: srv.URL, APIKey: "k", Model: "m"}).StreamChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, func(s string) error {
		got.WriteString(s)
		return nil
	})
	if err != nil || got.String() != "Halo" {
		t.Fatalf("got %q, err %v", got.String(), err)
	}
}

func TestStreamChatMapsProviderStatus(t *testing.T) {
	for _, status := range []int{401, 404, 429, 500} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, `{"error":"Authorization: Bearer k"}`, status)
		}))
		err := NewClient(Options{BaseURL: srv.URL, APIKey: "k", Model: "m"}).StreamChat(context.Background(), []Message{{Role: "user", Content: "hi"}}, func(string) error { return nil })
		srv.Close()

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("status %d: want APIError, got %v", status, err)
		}
		if strings.Contains(apiErr.Message, "Bearer") {
			t.Fatalf("status %d: provider body leaked into message", status)
		}
	}
}

func TestStreamChatWithoutKey(t *testing.T) {
	err := NewClient(Options{BaseURL: "http://127.0.0.1:1", APIKey: "", Model: "m"}).StreamChat(context.Background(), nil, nil)
	if !errors.Is(err, ErrNoAPIKey) {
		t.Fatalf("want ErrNoAPIKey, got %v", err)
	}
}

// hangingServer never answers. The release channel lets Close return even if
// the server never sees the client disconnect.
func hangingServer(t *testing.T) *httptest.Server {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	t.Cleanup(func() {
		close(release)
		srv.Close()
	})
	return srv
}

func TestStreamChatTimeout(t *testing.T) {
	srv := hangingServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := NewClient(Options{BaseURL: srv.URL, APIKey: "k", Model: "m"}).StreamChat(ctx, []Message{{Role: "user", Content: "hi"}}, func(string) error { return nil })

	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusGatewayTimeout {
		t.Fatalf("want gateway timeout, got %v", err)
	}
}

func TestStreamChatClientCancel(t *testing.T) {
	srv := hangingServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	err := NewClient(Options{BaseURL: srv.URL, APIKey: "k", Model: "m"}).StreamChat(ctx, []Message{{Role: "user", Content: "hi"}}, func(string) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestRateLimitMessages(t *testing.T) {
	cases := []struct {
		name, retryAfter, body, reset, want string
	}{
		{"retry-after", "30", "", "", "Coba lagi dalam 30 detik."},
		{"daily", "", `{"error":{"message":"daily quota exceeded"}}`, "tengah malam WIB", "Kuota harian provider sudah habis. Kuota direset tengah malam WIB."},
		{"burst", "", `{"error":{"message":"rate limit: 40 per minute"}}`, "", "tunggu sekitar satu menit"},
		{"unknown", "", `{}`, "", "Kalau masih gagal, kuota harian sudah habis. Coba lagi nanti."},
	}
	for _, tc := range cases {
		c := NewClient(Options{QuotaReset: tc.reset})
		if got := c.rateLimitMessage(tc.retryAfter, tc.body); !strings.Contains(strings.ToLower(got), strings.ToLower(tc.want)) {
			t.Errorf("%s: got %q, want it to contain %q", tc.name, got, tc.want)
		}
	}
}

func TestMessageJSON(t *testing.T) {
	text, _ := json.Marshal(Message{Role: "user", Content: "halo"})
	if string(text) != `{"role":"user","content":"halo"}` {
		t.Fatalf("pesan teks harus tetap string: %s", text)
	}

	withImg, _ := json.Marshal(Message{Role: "user", Content: "apa ini?", Images: []Image{{MIME: "image/jpeg", Data: []byte{1, 2, 3}}}})
	want := `{"role":"user","content":[{"type":"text","text":"apa ini?"},{"type":"image_url","image_url":{"url":"data:image/jpeg;base64,AQID"}}]}`
	if string(withImg) != want {
		t.Fatalf("got  %s\nwant %s", withImg, want)
	}

	onlyImg, _ := json.Marshal(Message{Role: "user", Images: []Image{{MIME: "image/png", Data: []byte{1}}}})
	if strings.Contains(string(onlyImg), `"type":"text"`) {
		t.Fatalf("teks kosong tidak boleh dikirim sebagai part: %s", onlyImg)
	}
}
