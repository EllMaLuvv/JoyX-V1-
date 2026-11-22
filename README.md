# JoyX v1

Bot WhatsApp self-hosted dengan asisten AI. Dibangun di atas [WazzapAgent-Go](https://github.com/Chomosuke9/WazzapAgent-Go): satu akun WhatsApp tertaut, balasan AI di grup dan chat pribadi, unduhan video otomatis, pembuat stiker, dan halaman lokal untuk memasang akun sekaligus membaca riwayat pesan.

## Fitur

**Balas AI**
- Jawab saat disebut (`@Joy`), saat namanya ditulis, atau saat dibalas pesannya — di grup maupun chat pribadi.
- Setelan trigger per chat: `/trigger mention|name|reply on|off`, plus smart rule untuk grup (`/trigger smart add ...`).
- Prompt sistem per chat (`/prompt`), level moderasi (`/permission`), dan ingatan jangka panjang (`/memory`).

**Unduh video otomatis**
- Tempel link TikTok, Instagram, Facebook atau X → video langsung diunduh dan dikirim, tanpa perintah.
- Atau ketik `/download <link>` (alias `/tt`, `/ig`, `/fb`, `/dl`).
- Resolusi terbaik dari sumber, sampai batas 1080p60; caption menyebut resolusi asli yang didapat.
- Story dan highlight Instagram (`instagram.com/stories/...`) didukung; story yang isinya foto dikirim sebagai gambar.
- Facebook *story* tidak bisa diambil (tidak didukung yt-dlp) — hanya video dan reel biasa.

**Stiker**
- `.s` / `/sticker` mengubah gambar, GIF atau video (yang dikirim atau di-quote) menjadi stiker WhatsApp.
- Teks meme opsional: `/sticker teks atas#teks bawah`.
- Semua stiker buatan Joy membawa watermark `JOYX` di sudutnya.
- Simpan stiker bernama dengan `/add-sticker <nama>`, kirim ulang lewat katalog chat.

**Local Host** — http://127.0.0.1:3001
- QR pairing, status bot, dan panel "Riwayat Joy" berisi pesan yang dikirim Joy dalam 7 hari terakhir.
- Logo dan latar halaman bisa diganti lewat `web/logo.jpg` dan `web/background.jpg`.

**Perintah lain yang tersedia**
- `/img` — kirim balik stiker/gambar sebagai gambar. `/rvo` (owner) — buka kembali view-once.
- `/dd <nama> <tanggal>` — ingatkan deadline sehari sebelumnya.
- `/schedule-task` dan `/daily-task` — pengingat sekali/jadwal harian.
- `/group` — kelola grup (tutup/buka, deskripsi, hapus, bisukan, kick) sesuai izin.
- `/del`, `/restart` (owner), serta bentuk titik: `.dd-…`, `.s`, `.img`, `.rvo`, `.del`, `.reset`.

## Persyaratan

- Go (untuk membangun) dan akun WhatsApp yang akan ditautkan.
- Endpoint chat-completions yang kompatibel OpenAI + API key-nya.
- `yt-dlp` dan `ffmpeg` untuk unduhan video. Di Windows `curl_cffi` wajib terpasang pada interpreter Python yang dipakai yt-dlp — tanpa itu TikTok menolak request.
- Browser untuk membuka halaman lokal saat memasang akun.

## RUN

```sh
go build -o bin/joyx.exe ./cmd/wazzapagent
./bin/joyx.exe
```

Windows: jalankan `run-joyx.bat` (build otomatis kalau binary belum ada). Buka http://127.0.0.1:3001 lalu pindai QR dari WhatsApp → Perangkat Tertaut. Sesi tersimpan di `data/`, jadi restart berikutnya tidak perlu memindai lagi.

## Konfigurasi

Selengkapnya di `.env.example`. Yang paling penting:

| Variabel | Function |
| --- | --- |
| `WAZZAP_LLM_ENDPOINT`, `WAZZAP_LLM_API_KEY`, `WAZZAP_LLM_MODEL` | Penyedia model (contoh: Groq + `openai/gpt-oss-120b`) |
| `WAZZAP_OWNER` | JID/nomor pemilik |
| `WAZZAP_YT_DLP_PATH`, `WAZZAP_FFMPEG_PATH` | Jalur absolut alat kalau tidak ada di PATH |
| `WAZZAP_YT_DLP_COOKIES_FROM` | Browser berisi login Instagram/TikTok (Firefox paling mudah di Windows; cookie Chrome terenkripsi) |
| `WAZZAP_DOWNLOAD_MAX_MB` | Batas ukuran unduhan |
| `WAZZAP_PAIRING_WEB` | Alamat halaman lokal, `off` untuk mematikan |

## Struct

- `internal/inbound/commands/` — satu perintah satu file (`download.go`, `sticker.go`, `memory.go`, …).
- `internal/downloader/` — pemilihan format, penggabungan audio, dan batas ukuran.
- `internal/sticker/` — konversi ke WebP 512×512, teks meme, dan watermark `JOYX`.
- `internal/adapters/whatsapp/hypermeow/` — integrasi WhatsApp, halaman pairing, dan panel riwayat.
- `internal/app/systemprompt.txt` — instruksi yang dibaca Joy tiap giliran.

## Data

Semua state lokal ada di `data/` (database SQLite, sesi WhatsApp, cache). `.env` berisi kredensial dan **tidak** ikut versi kode — jangan pernah di-commit.
