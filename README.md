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
cp .env.example .env      # lalu isi LLM_API_KEY
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
| GET | `/api/health` | Status server dan apakah API key sudah diisi |
| POST | `/api/chat` | Body `{"messages":[{"role":"user","content":"..."}]}`, balasan SSE |

Event SSE dari `/api/chat`: `token` (`{"content":"..."}`), `done`, dan `error` (`{"message":"..."}`).

## Cek sebelum commit

```bash
cd backend && go vet ./... && go test ./... && go build ./...
cd frontend && npm run build
```

## Ikon PWA

Ikon dibuat dari `frontend/public/mascot.svg`. Kalau SVG maskot berubah, buat ulang ikonnya:

```bash
cd frontend && npm run icons
```

Renderer ikon (sharp) tidak mengenal CSS variable, jadi `scripts/build-icons.mjs` merender dari salinan yang nilai `var()`-nya sudah diganti warna tema terang. `mascot.svg` sendiri tidak diubah.
