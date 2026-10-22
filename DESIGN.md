# DESIGN.md: Boo Dou

Arah desain untuk semua kerja UI. Dokumen ini berisi keputusan pemilik produk. Aturan AntiSlop di `../anti-slop` dipakai sebagai filter di atasnya.

Dial: ENERGY 3 / RHYTHM 2 / MOTION 3

Design Read: aplikasi chat asisten AI untuk pengguna berbahasa Indonesia, dengan bahasa visual kertas washi dan tinta sumi yang tertata seperti seorang gentleman, dial ENERGY 3 / RHYTHM 2 / MOTION 3.

## Identitas

Boo Dou adalah asisten AI berwujud hantu kecil bertopi fedora. Personanya seorang gentleman: sopan, tenang, tertata, dengan sisi bersemangat yang muncul sesekali. Sumber persona dan cara bicaranya ada di `backend/internal/persona/boo_dou.md`. Teks UI ditulis dengan suara yang sama.

## Palet

Sumber arah: palet tradisional Jepang, yaitu kertas washi dan tinta sumi sebagai netral, hijau teh sebagai warna inti, dan vermilion (shu) sebagai satu aksen yang berani untuk ENERGY 3. Netral tidak dihitung, jadi palet aktifnya 2 inti (hijau tua dan tint hijau teh) dan 1 aksen (vermilion).

### Tema terang (default)

| Token | Hex | Peran |
|---|---|---|
| `--color-bg` | `#F3EFE4` | Latar, kertas washi |
| `--color-surface` | `#FBF9F3` | Permukaan bubble asisten, dialog, input |
| `--color-ink` | `#22201C` | Teks utama, tinta sumi |
| `--color-muted` | `#5C584F` | Teks sekunder, label "Dihentikan", waktu |
| `--color-border` | `#8C8677` | Border input dan pemisah yang wajib terlihat |
| `--color-green` | `#3F5A35` | Inti: header, judul, link, focus ring, teks di atas tint |
| `--color-tea` | `#DCE3C6` | Inti (tint): bubble user, item percakapan aktif |
| `--color-accent` | `#B3402A` | Aksen: hanya tombol kirim. Teks di atasnya `#FFFFFF` |

### Tema gelap

| Token | Hex | Peran |
|---|---|---|
| `--color-bg` | `#171614` | Latar |
| `--color-surface` | `#22201C` | Permukaan |
| `--color-ink` | `#EDE8DC` | Teks utama |
| `--color-muted` | `#B4AD9E` | Teks sekunder |
| `--color-border` | `#76705F` | Border wajib terlihat |
| `--color-green` | `#A3B88E` | Inti |
| `--color-tea` | `#2D3A28` | Bubble user, teks `--color-ink` di atasnya |
| `--color-accent` | `#E07A5F` | Aksen, teks di atasnya `#171614` |

### Kontras

Dihitung ulang dengan rumus WCAG 2.x (sama dengan `contrast-check.py` di skill antislop-human). Semua angka pemilik cocok sampai dua desimal.

| Pasangan | Terang | Gelap | Syarat |
|---|---|---|---|
| ink / bg | 14.15 | 14.79 | 4.5 |
| ink / surface | 15.45 | 13.30 | 4.5 |
| muted / bg | 6.17 | 8.10 | 4.5 |
| muted / surface | 6.73 | 7.29 | 4.5 |
| ink / tea | 12.27 | 9.83 | 4.5 |
| green / bg | 6.71 | 8.44 | 4.5 |
| green / surface | 7.32 | 7.59 | 4.5 |
| teks di atas accent / accent | 5.70 | 6.13 | 4.5 |
| accent / bg | 4.96 | 6.13 | 3 |
| border / bg | 3.16 | 3.66 | 3 |
| border / surface | 3.44 | 3.29 | 3 |
| surface (terang) atau bg (gelap) di atas green | 7.32 | 8.44 | 4.5 |
| muted / tea | 5.35 | 5.39 | 4.5 |
| green / tea | 5.81 | 5.61 | 3 |
| border / tea | 2.74 | 2.43 | 3, gagal |

Batasan dari tabel ini: `--color-border` tidak boleh menjadi satu-satunya batas elemen di atas `--color-tea`. Elemen bergaris di atas tint (misalnya input rename di item percakapan aktif) memakai `--color-green` sebagai garis.

`--color-surface` dan `--color-tea` hampir sama dengan `--color-bg` (1.09:1 dan 1.15:1 di terang). Bubble karena itu tidak boleh dibedakan dari latar hanya lewat warna; pembedanya posisi dan tipografi.

### Aturan pakai

- Aksen vermilion hanya untuk tombol kirim. Focus ring memakai `--color-green` (3:1 terpenuhi di dua tema), bukan aksen, supaya aksen tidak menyebar.
- Tidak ada warna merah terpisah untuk error atau hapus. Error dan aksi berbahaya dibedakan lewat teks, ikon, dan posisi, bukan lewat warna.
- Maskot: pita topi `#4F7A3A` supaya senada dengan hijau teh. Kontras pita vs topi 3.47 (terang) dan 3.88 (gelap). Outline maskot vs `--color-bg`: 13.58 terang, 15.11 gelap.
- `theme_color` manifest PWA memakai `--color-green` tema terang (`#3F5A35`).

## Tipografi

Satu keluarga sans berkarakter untuk semua teks: Plus Jakarta Sans (variable, self-host lewat `@fontsource-variable/plus-jakarta-sans` supaya jalan offline di PWA). Alasan: font ini dibuat untuk identitas kota Jakarta, cocok untuk aplikasi berbahasa Indonesia, dan punya karakter tanpa jadi font default AI.

Kode tetap memakai monospace bawaan sistem, dengan ligatur dimatikan supaya yang tampil sama dengan yang tersalin.

## Dial dan tuas

- ENERGY 3 (berani), RHYTHM 2 (konsisten dengan beberapa jeda), MOTION 3 (koreografi).
- Satu titik fokus per layar.
- Satu aksen: vermilion di tombol kirim.
- Whitespace sebagai struktur.

## Motif identitas

Lengkung brim fedora: tepi depan yang turun di tengah. Dipakai di 1 sampai 2 tempat yang punya alasan, misalnya tepi atas bar input atau pemisah header.

Bintang 4 sudut di pita topi tidak dipakai sebagai ikon UI, karena termasuk pola ikon sparkle generik.

## Tema

Terang dan gelap sama-sama jalan penuh. Tema terang jadi default karena persona gentleman dan nuansa kertas washi lebih kuat di terang. Tema gelap ikut `prefers-color-scheme` kalau user belum memilih.
