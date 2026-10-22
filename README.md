# Boo Dou

Asisten AI berbasis web. Backend Go + Gin meneruskan chat ke provider LLM yang OpenAI-compatible dan mengalirkan jawabannya ke frontend Vue 3 lewat SSE.

## Kebutuhan

- Go 1.22 atau lebih baru
- Node 20 atau lebih baru
- air untuk hot reload backend: `go install github.com/air-verse/air@latest`

Pastikan `C:\Program Files\Go\bin`, `C:\Program Files\nodejs`, dan `%USERPROFILE%\go\bin` ada di PATH. Di shell MSYS2 UCRT64, path yang sama ditulis `/c/Program Files/Go/bin` dan seterusnya.

## Setup

```bash
cd backend
cp .env.example .env      # lalu isi LLM_API_KEY dan DATABASE_URL
go mod download

cd ../frontend
npm install
```

Untuk pindah provider atau ke paket berbayar, ganti `LLM_BASE_URL`, `LLM_API_KEY`, dan `LLM_MODEL` di `backend/.env`. Kode tidak perlu diubah. `LLM_QUOTA_RESET` (opsional) dipakai di pesan error saat kuota harian habis.

Key free tier Bansos berlaku terbatas. Kalau chat mulai membalas "API key ditolak provider", buat key baru dan ganti `LLM_API_KEY`.

## Menjalankan dev

Dua terminal:

```bash
cd backend && air          # http://localhost:8080
cd frontend && npm run dev # http://localhost:5173
```

Tanpa air, jalankan backend dengan `go run ./cmd/server`.

Di VS Code, jalankan task `dev: semua` (Terminal > Run Task) atau konfigurasi debug `Full stack` untuk Delve + Chrome sekaligus.

## Endpoint

| Method | Path | Keterangan |
|---|---|---|
| GET | `/api/health` | Status, apakah API key terisi, dan `llm_vision` |
| GET, POST | `/api/conversations` | Daftar (terbaru dulu) dan buat percakapan |
| GET, PATCH, DELETE | `/api/conversations/:id` | Isi beserta pesan dan lampiran, ganti judul, hapus (file gambar ikut terhapus) |
| POST | `/api/conversations/:id/chat` | Body `{"text":"...","attachment_ids":[]}` atau `{"retry":true}`, balasan SSE |
| POST | `/api/attachments` | Multipart field `file`: JPEG/PNG/WebP, maks 5 MB |
| GET | `/api/attachments/:id` | File gambar |

Event SSE dari endpoint chat: `token` (`{"content":"..."}`), `done` (`{"message_id":"..."}`), dan `error` (`{"message":"..."}`).

## Database

PostgreSQL dari `DATABASE_URL`. Migrasi di `backend/internal/db/migrations` jalan otomatis saat backend start. `go test` memakai `DATABASE_URL_TEST`, yang dikosongkan setiap tes jalan, jadi nama databasenya wajib berakhiran `_test`. Kalau variabel itu kosong, tes database dilewati.

Gambar disimpan di `UPLOAD_DIR` (default `backend/data/uploads`, tidak masuk git). Frontend mengompres gambar ke sisi terpanjang 1568 px sebelum upload, yang sekaligus membuang EXIF termasuk lokasi GPS.

Set `LLM_VISION=true` hanya kalau `LLM_MODEL` bisa membaca gambar.

## Cek sebelum commit

```bash
cd backend && go vet ./... && go test ./... && go build ./...
cd frontend && npm run build && npm test
```

## Ikon PWA

Ikon dibuat dari `frontend/public/mascot.svg`. Kalau SVG maskot berubah, buat ulang ikonnya:

```bash
cd frontend && npm run icons
```

Renderer ikon (sharp) tidak mengenal CSS variable, jadi `scripts/build-icons.mjs` merender dari salinan yang nilai `var()`-nya sudah diganti warna tema terang. `mascot.svg` sendiri tidak diubah.
