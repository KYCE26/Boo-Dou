package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"boodou/backend/internal/llm"
	"boodou/backend/internal/storage"
	"boodou/backend/internal/store"
	"boodou/backend/internal/testdb"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// fakeProvider records every request body and answers according to mode.
type fakeProvider struct {
	mu     sync.Mutex
	bodies []string
	mode   string // "ok", "500", "400", "hang"
	srv    *httptest.Server
}

func newFakeProvider(t *testing.T) *fakeProvider {
	p := &fakeProvider{mode: "ok"}
	p.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		p.mu.Lock()
		p.bodies = append(p.bodies, string(b))
		mode := p.mode
		p.mu.Unlock()

		switch mode {
		case "500":
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		case "400":
			http.Error(w, "bad image", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"Jawaban "}}]}`+"\n\n")
		w.(http.Flusher).Flush()
		if mode == "hang" {
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"uji."}}]}`+"\n\ndata: [DONE]\n\n")
	}))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *fakeProvider) set(mode string) { p.mu.Lock(); p.mode = mode; p.mu.Unlock() }

func (p *fakeProvider) last() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.bodies) == 0 {
		return ""
	}
	return p.bodies[len(p.bodies)-1]
}

func (p *fakeProvider) count() int { p.mu.Lock(); defer p.mu.Unlock(); return len(p.bodies) }

type env struct {
	pool     *pgxpool.Pool
	router   *gin.Engine
	store    *store.Store
	provider *fakeProvider
	dir      string
}

func newEnv(t *testing.T, vision bool, apiKey string) *env {
	gin.SetMode(gin.TestMode)
	pool := testdb.Open(t)
	st := store.New(pool)
	dir := t.TempDir()
	files, err := storage.NewLocal(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := newFakeProvider(t)
	client := llm.NewClient(llm.Options{BaseURL: p.srv.URL, APIKey: apiKey, Model: "model-uji"})

	r := gin.New()
	convs := NewConversations(st, files)
	r.GET("/api/conversations", convs.List)
	r.POST("/api/conversations", convs.Create)
	r.GET("/api/conversations/:id", convs.Get)
	r.PATCH("/api/conversations/:id", convs.Rename)
	r.DELETE("/api/conversations/:id", convs.Delete)
	r.POST("/api/conversations/:id/chat", NewConversationChat(st, files, client, "model-uji", vision).Handle)
	atts := NewAttachments(st, files, vision)
	r.POST("/api/attachments", atts.Upload)
	r.GET("/api/attachments/:id", atts.Serve)
	return &env{pool: pool, router: r, store: st, provider: p, dir: dir}
}

func (e *env) do(method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *env) upload(t *testing.T, name string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", name)
	_, _ = fw.Write(data)
	_ = mw.Close()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/attachments", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	e.router.ServeHTTP(rec, req)
	return rec
}

func (e *env) uploadPNG(t *testing.T, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)))
	rec := e.upload(t, "foto.png", buf.Bytes())
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload png: %d %s", rec.Code, rec.Body.String())
	}
	var a struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &a)
	return a.ID
}

func (e *env) newConversation(t *testing.T) string {
	t.Helper()
	rec := e.do(http.MethodPost, "/api/conversations", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var c struct{ ID string }
	_ = json.Unmarshal(rec.Body.Bytes(), &c)
	return c.ID
}

func (e *env) chat(convID, body string) *httptest.ResponseRecorder {
	return e.do(http.MethodPost, "/api/conversations/"+convID+"/chat", body)
}

func (e *env) messages(t *testing.T, convID string) []store.Message {
	t.Helper()
	_, msgs, err := e.store.GetConversation(context.Background(), convID)
	if err != nil {
		t.Fatal(err)
	}
	return msgs
}

func TestConversationCRUD(t *testing.T) {
	e := newEnv(t, true, "k")
	id := e.newConversation(t)

	if rec := e.do(http.MethodGet, "/api/conversations", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), id) {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	if rec := e.do(http.MethodPatch, "/api/conversations/"+id, `{"title":"   "}`); rec.Code != 400 {
		t.Fatalf("judul kosong harus 400, got %d", rec.Code)
	}
	if rec := e.do(http.MethodPatch, "/api/conversations/"+id, `{"title":"  Judul   baru "}`); rec.Code != 200 || !strings.Contains(rec.Body.String(), `"title":"Judul baru"`) {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	rec := e.do(http.MethodGet, "/api/conversations/"+id, "")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"messages":[]`) {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	if rec := e.do(http.MethodDelete, "/api/conversations/"+id, ""); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	for _, path := range []string{"/api/conversations/" + id, "/api/conversations/bukan-uuid"} {
		if rec := e.do(http.MethodGet, path, ""); rec.Code != 404 {
			t.Fatalf("GET %s: want 404, got %d", path, rec.Code)
		}
	}
}

func TestUploadValidation(t *testing.T) {
	e := newEnv(t, true, "k")

	if rec := e.upload(t, "catatan.png", []byte("ini teks biasa, bukan gambar")); rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("txt menyamar png: want 415, got %d %s", rec.Code, rec.Body.String())
	}
	big := make([]byte, MaxAttachmentBytes+10)
	copy(big, []byte("\x89PNG\r\n\x1a\n"))
	if rec := e.upload(t, "besar.png", big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("lebih dari 5 MB: want 413, got %d", rec.Code)
	}

	id := e.uploadPNG(t, 3, 2)
	rec := e.do(http.MethodGet, "/api/attachments/"+id, "")
	h := rec.Header()
	if rec.Code != 200 || h.Get("Content-Type") != "image/png" || h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Cache-Control") != "private" {
		t.Fatalf("serve: %d %v", rec.Code, h)
	}
	a, _ := e.store.GetAttachment(context.Background(), id)
	if a.Width != 3 || a.Height != 2 || a.StorageKey != id+".png" {
		t.Fatalf("metadata: %+v", a)
	}
	if _, err := os.Stat(filepath.Join(e.dir, a.StorageKey)); err != nil {
		t.Fatalf("file harus tersimpan dengan nama dari id: %v", err)
	}

	off := newEnv(t, false, "k")
	if rec := off.upload(t, "x.png", []byte("x")); rec.Code != http.StatusForbidden {
		t.Fatalf("LLM_VISION false: want 403, got %d", rec.Code)
	}
}

func TestChatSavesHistoryAndLimitsImages(t *testing.T) {
	e := newEnv(t, true, "k")
	conv := e.newConversation(t)
	img1, img2, img3 := e.uploadPNG(t, 2, 2), e.uploadPNG(t, 2, 2), e.uploadPNG(t, 2, 2)

	for i, img := range []string{img1, img2, img3} {
		rec := e.chat(conv, fmt.Sprintf(`{"text":"gambar ke-%d","attachment_ids":["%s"]}`, i+1, img))
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), "event: done") {
			t.Fatalf("chat %d: %d %s", i+1, rec.Code, rec.Body.String())
		}
	}

	var sent struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(e.provider.last()), &sent); err != nil {
		t.Fatal(err)
	}
	// persona + 3 user + 2 assistant
	if len(sent.Messages) != 6 || sent.Messages[0].Role != "system" {
		t.Fatalf("susunan pesan ke provider: %d pesan", len(sent.Messages))
	}
	first, second, third := string(sent.Messages[1].Content), string(sent.Messages[3].Content), string(sent.Messages[5].Content)
	if strings.Contains(first, "image_url") || !strings.Contains(first, olderImageNote) {
		t.Fatalf("gambar lama harus diganti catatan: %s", first)
	}
	for _, c := range []string{second, third} {
		if !strings.Contains(c, `"type":"image_url"`) || !strings.Contains(c, "data:image/png;base64,") {
			t.Fatalf("2 pesan bergambar terakhir harus membawa gambar: %.120s", c)
		}
	}

	msgs := e.messages(t, conv)
	if len(msgs) != 6 || msgs[1].Status != "complete" || msgs[1].Content != "Jawaban uji." || msgs[1].Model != "model-uji" {
		t.Fatalf("riwayat tersimpan: %+v", msgs)
	}
	conversation, _, _ := e.store.GetConversation(context.Background(), conv)
	if conversation.Title != "gambar ke-1" {
		t.Fatalf("judul: %q", conversation.Title)
	}
	if strings.Contains(conversation.Title, "Kamu adalah Boo Dou") {
		t.Fatal("system prompt tidak boleh disimpan")
	}
}

func TestChatRejectsWithoutSaving(t *testing.T) {
	cases := []struct {
		name, body string
		vision     bool
		key        string
		want       int
	}{
		{"vision mati", `{"text":"x","attachment_ids":["%s"]}`, false, "k", 400},
		{"lebih dari 4", `{"text":"x","attachment_ids":["%s","a","b","c","d"]}`, true, "k", 400},
		{"lampiran tidak dikenal", `{"text":"x","attachment_ids":["00000000-0000-4000-8000-000000000000"]}`, true, "k", 400},
		{"key kosong", `{"text":"x"}`, true, "", 503},
		{"pesan kosong", `{"text":"   "}`, true, "k", 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t, true, tc.key)
			img := e.uploadPNG(t, 1, 1)
			e = &env{pool: e.pool, router: newEnvRouter(t, e, tc.vision, tc.key), store: e.store, provider: e.provider, dir: e.dir}
			conv := e.newConversation(t)
			body := tc.body
			if strings.Contains(body, "%s") {
				body = fmt.Sprintf(body, img)
			}
			rec := e.chat(conv, body)
			if rec.Code != tc.want || !strings.Contains(rec.Body.String(), "event: error") {
				t.Fatalf("want %d + event error, got %d %s", tc.want, rec.Code, rec.Body.String())
			}
			if n := len(e.messages(t, conv)); n != 0 {
				t.Fatalf("tidak boleh ada pesan tersimpan, ada %d", n)
			}
			if e.provider.count() != 0 {
				t.Fatal("provider tidak boleh dipanggil")
			}
		})
	}
}

// newEnvRouter rebuilds the router on the same store and files with other
// vision/key settings, so an upload made with vision on can be reused.
func newEnvRouter(t *testing.T, base *env, vision bool, key string) *gin.Engine {
	files, _ := storage.NewLocal(base.dir)
	client := llm.NewClient(llm.Options{BaseURL: base.provider.srv.URL, APIKey: key, Model: "model-uji"})
	r := gin.New()
	convs := NewConversations(base.store, files)
	r.POST("/api/conversations", convs.Create)
	r.POST("/api/conversations/:id/chat", NewConversationChat(base.store, files, client, "model-uji", vision).Handle)
	return r
}

func TestChatErrorThenRetry(t *testing.T) {
	e := newEnv(t, true, "k")
	conv := e.newConversation(t)

	e.provider.set("500")
	rec := e.chat(conv, `{"text":"halo"}`)
	if !strings.Contains(rec.Body.String(), "event: error") {
		t.Fatalf("want error event: %s", rec.Body.String())
	}
	msgs := e.messages(t, conv)
	if len(msgs) != 2 || msgs[1].Status != "error" {
		t.Fatalf("jawaban gagal harus tersimpan dengan status error: %+v", msgs)
	}

	e.provider.set("ok")
	rec = e.chat(conv, `{"retry":true}`)
	if !strings.Contains(rec.Body.String(), "event: done") {
		t.Fatalf("retry: %s", rec.Body.String())
	}
	msgs = e.messages(t, conv)
	if len(msgs) != 2 || msgs[0].Content != "halo" || msgs[1].Status != "complete" {
		t.Fatalf("retry tidak boleh menggandakan pesan user dan harus mengganti jawaban error: %+v", msgs)
	}
	if strings.Count(e.provider.last(), `"content":"halo"`) != 1 {
		t.Fatal("provider menerima pesan user ganda")
	}
}

func TestChatImageRejectedByProvider(t *testing.T) {
	e := newEnv(t, true, "k")
	conv := e.newConversation(t)
	img := e.uploadPNG(t, 1, 1)
	e.provider.set("400")
	rec := e.chat(conv, fmt.Sprintf(`{"text":"apa ini","attachment_ids":["%s"]}`, img))
	if !strings.Contains(rec.Body.String(), "Provider menolak gambar") {
		t.Fatalf("pesan penolakan gambar harus jelas: %s", rec.Body.String())
	}
}

func TestChatStoppedKeepsPartialAnswer(t *testing.T) {
	e := newEnv(t, true, "k")
	conv := e.newConversation(t)
	e.provider.set("hang")
	srv := httptest.NewServer(e.router)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/api/conversations/"+conv+"/chat", strings.NewReader(`{"text":"cerita panjang"}`))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "event: token") {
			break
		}
	}
	cancel() // same as the browser's Hentikan button
	resp.Body.Close()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msgs := e.messages(t, conv)
		if len(msgs) == 2 {
			if msgs[1].Status != "stopped" || msgs[1].Content != "Jawaban " {
				t.Fatalf("jawaban terpotong: %+v", msgs[1])
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("jawaban stopped tidak tersimpan")
}

func TestDeleteAndCleanupRemoveFiles(t *testing.T) {
	e := newEnv(t, true, "k")
	conv := e.newConversation(t)
	img := e.uploadPNG(t, 2, 2)
	if rec := e.chat(conv, fmt.Sprintf(`{"text":"lihat","attachment_ids":["%s"]}`, img)); rec.Code != 200 {
		t.Fatalf("chat: %d", rec.Code)
	}
	file := filepath.Join(e.dir, img+".png")
	if _, err := os.Stat(file); err != nil {
		t.Fatal(err)
	}
	if rec := e.do(http.MethodDelete, "/api/conversations/"+conv, ""); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("file gambar harus ikut terhapus dari disk")
	}

	orphan := e.uploadPNG(t, 1, 1)
	_, _ = e.pool.Exec(context.Background(), `UPDATE attachments SET created_at = now() - interval '2 days' WHERE id = $1`, orphan)
	files, _ := storage.NewLocal(e.dir)
	if n, err := CleanupOrphans(context.Background(), e.store, files); err != nil || n != 1 {
		t.Fatalf("cleanup: %d %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(e.dir, orphan+".png")); !os.IsNotExist(err) {
		t.Fatal("file lampiran yatim harus terhapus")
	}
}
