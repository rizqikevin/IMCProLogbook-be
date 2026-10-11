# Machine Logbook Archive · React

Frontend React (JavaScript) + Vite, tersambung langsung ke REST API Go.

## Jalankan lokal

Jalankan `docker compose up -d --build` dari root terlebih dahulu. Dari direktori `frontend`:

```sh
npm ci
npm run dev
```

Buka `http://localhost:5173`, lalu masuk menggunakan akun operator atau admin backend.
Tidak ada akun/password default yang ditambahkan ke database aplikasi.

Vite meneruskan `/api` ke `http://127.0.0.1:18472`. Jika alamat backend berbeda,
salin `.env.example` menjadi `.env.local` lalu ubah `API_PROXY_TARGET`.
Untuk akses lintas domain, set `VITE_API_BASE_URL` ke origin backend dan tambahkan
origin frontend pada `CORS_ALLOWED_ORIGINS` backend. Nilai `VITE_*` menjadi bagian
bundle publik, sehingga tidak boleh berisi password atau secret.

Jika perlu membuat akun, jalankan dari direktori backend:

```sh
docker compose exec app /bin/logbook user create --username operator1 --name "Operator 1" --role operator
```

Password diminta secara interaktif. Gunakan `--role admin` untuk administrator.

## Alur penggunaan

1. Login dengan akun operator atau admin.
2. Pilih mesin untuk membuka satu riwayat percakapan yang berkelanjutan.
3. Ketik atau tempel catatan multiline, pilih foto, atau gunakan keduanya.
4. Tekan **Kirim pesan**. Nama pengirim dan waktu ditentukan server; tanggal dan shift tidak diperlukan.
5. Gunakan **Muat pesan sebelumnya** untuk membaca pesan lama. Pesan tampil dari lama ke baru.
6. **Pilih mesin lain** kembali ke daftar mesin. Draft teks dan foto masing-masing mesin tetap ada selama tab dan sesi yang sama.
7. Tekan **Cari pesan**, isi teks, lalu tekan **Cari** atau Enter. Pencarian mencakup seluruh riwayat teks mesin ini, bukan hanya pesan yang sudah dimuat. **Muat hasil sebelumnya** membuka kecocokan yang lebih lama.
8. **Reset**, **Tutup pencarian**, atau Escape mengembalikan riwayat lengkap. Kata pencarian yang sudah dikirim tersimpan pada URL `?q=`, sehingga reload dan tombol Back/Forward memulihkan filter. Mengetik saja tidak mengubah hasil. Draft pesan dan foto tidak berubah saat mencari.

Teks ditampilkan sebagai teks biasa, bukan HTML atau Markdown. Baris baru, spasi,
dan tab tetap disimpan. Satu kiriman menjadi satu pesan; tidak ada parser WhatsApp,
impor, sinkronisasi realtime, atau edit pesan.

Arsip lama tampil sebagai arsip foto dengan tanggal dan shift aslinya, tanpa teks
buatan. Tautan `/logbooks/:id` dan navigasi shift lama tetap dapat dibaca. Form
pembuatan arsip lama dan penambahan halaman tidak lagi menjadi route frontend.
Admin tetap dapat menghapus arsip lama melalui detail; backend memverifikasi izin.

## Penyimpanan dan kegagalan pengiriman

- Draft teks dan foto hanya berada di memori tab. Berpindah mesin tidak menghapusnya;
  refresh, menutup tab, atau keluar akun menghapusnya. Browser memberi peringatan
  saat meninggalkan tab dengan draft. Tidak ada penyimpanan offline.
- Kegagalan pengiriman mempertahankan draft. Tidak ada pengiriman ulang otomatis.
  Jika koneksi terputus setelah server mungkin menyimpan pesan, gunakan **Muat ulang
  riwayat** dan periksa hasil sebelum mengirim ulang agar tidak membuat duplikat.
- Saat mengirim, teks dan pilihan foto terkunci. Pengiriman tetap berjalan jika
  pengguna membuka mesin lain; hasil dan draft tetap terkait mesin asal.
- Maksimal 20.000 karakter Unicode per pesan. Pesan memerlukan teks tidak kosong
  atau setidaknya satu foto. Teks tidak dipangkas sebelum dikirim.
- Batas foto mengikuti default backend: 20 foto per request, 10 MiB/foto,
  40 megapiksel, dan total foto maksimal 95.000.000 byte. JPG, PNG, dan WebP
  diterima; HEIC harus dikonversi dahulu. Foto perangkat tidak dikompresi ulang.
- Token sesi berada di `sessionStorage` atau memori jika storage tidak tersedia.
  Password tidak disimpan. Foto dimuat lewat helper Bearer-token sebagai blob,
  termasuk URL foto arsip lama; blob URL dilepas saat tidak digunakan.

## Kontrak percakapan

Route utama: `/` untuk pilihan mesin, `/machines/:machineId` untuk percakapan.
`GET /api/v1/machines/{machineID}/messages?cursor=&limit=&q=` mengembalikan
`{items, next_cursor?}`. Parameter kosong dihilangkan. Halaman awal berisi pesan
terbaru dalam urutan waktu menaik; cursor opaque mengambil halaman lebih lama.
`q` mencari potongan teks literal tanpa membedakan huruf besar/kecil, maksimal
200 karakter Unicode setelah spasi tepi dipangkas. `%`, `_`, dan backslash bukan
wildcard. Query kosong berarti riwayat lengkap; parameter `q` berulang, UTF-8
tidak valid, dan NUL ditolak API. Pencarian tidak mencakup nama pengirim, nama
file, atau isi foto. Foto pada pesan yang cocok tetap dapat dibuka. Arsip lama
dan pesan foto tanpa teks muncul kembali setelah filter dihapus.
Cursor terikat pada mesin dan query yang sama. Frontend membatalkan request lama,
mereset pagination saat filter berubah, dan memuat ulang hasil setelah mengirim
pesan saat pencarian aktif agar pesan yang tidak cocok tidak masuk ke hasil.
`POST` pada route yang sama menggunakan multipart dengan `text` opsional dan
field `photos` berulang. Respons berisi pesan lengkap dengan `author`, `created_at`,
dan `photos`; `legacy_archive` mengacu pada arsip foto historis.

## Build dan Docker

```sh
npm run build
npm run preview
```

`preview` hanya melihat hasil build dan tidak menyediakan proxy development. Gunakan
alamat API yang sesuai saat build, atau jalankan Docker berikut untuk integrasi API.

Dari direktori backend:

```sh
docker compose up --build -d
```

Frontend ada di `http://localhost:18472`. Server Go meneruskan `/api` ke service `app` dan
menangani deep link React Router. Tambahkan TLS pada ingress untuk penggunaan production
dan kamera ponsel. `WEB_PORT` dapat mengganti port lokal frontend.

Untuk Linux home server dengan Cloudflare Tunnel tanpa Nginx, ikuti
[panduan home server](../docs/home-server.md). Mac dan Linux memakai `docker-compose.yml` yang sama.

## Struktur

```text
src/
  auth.jsx           sesi dan proteksi route
  hooks.js           pembacaan master data, arsip, dan foto
  lib/api.js         fetch, Bearer token, error, multipart, foto terautentikasi
  lib/messages.js    validasi teks dan penggabungan halaman pesan
  components/        layout, dialog, draft mesin, foto pesan
  pages/             Login, Archives (pilihan mesin), Conversation, Book, ShiftArchive
  styles.css         desain responsive tanpa UI framework
e2e/
  backend/main.go    backend test dengan schema PostgreSQL terisolasi
  logbook.spec.js    pengujian browser terhadap backend nyata
server/
  main.go            server file React dan proxy API untuk container production
  main_test.go       pengujian routing SPA, cache, header, dan proxy upload
```

## Pengujian

```sh
npm test
npm run build
go test -race ./server
npx playwright install chromium
TEST_DATABASE_URL='postgres://user:password@localhost:5432/test_database?sslmode=disable' npm run test:e2e
```

E2E membutuhkan Go dan PostgreSQL test. Runner membuat schema terpisah, akun test, dan
folder foto sementara; semuanya dibersihkan saat server test berhenti normal. Jangan
gunakan database production. Pengujian mencakup teks multiline yang di-escape,
foto saja, teks dengan foto, persistensi, arsip historis, pagination, draft antar
mesin, kegagalan jaringan, loading/error/empty, axe, dan overflow desktop/ponsel.
Kasus pencarian mencakup kecocokan di luar halaman awal, pagination hasil,
isolasi mesin, literal `%`/`_`, foto, query URL/reload/Back/Forward, pembatalan
request lama, validasi panjang, hasil kosong, retry, dan kiriman di luar filter.

Desain disepakati dalam `DESIGN.md`. Pengujian perangkat Android/iOS, keyboard
ponsel, dan zoom 200% tetap diperlukan sebelum deployment.

## Install di ponsel (PWA)

Buka domain HTTPS aplikasi. Di Android Chrome, pilih menu **Install app / Tambahkan
ke layar utama**. Di iPhone Safari, pilih **Bagikan → Tambahkan ke Layar Utama**,
aktifkan **Buka sebagai App** bila tersedia, lalu tambah.

Manifest, ikon Android/maskable, dan Apple touch icon disertakan dalam build.
Service worker hanya aktif pada build production, bukan Vite development.
Gunakan Docker untuk pengujian yang sama dengan deployment.

Aplikasi tetap membutuhkan internet untuk login, membaca arsip, dan mengunggah.
Cache PWA hanya menyimpan halaman offline beserta CSS-nya; API, foto, dan token
login tidak dimasukkan ke Cache Storage. Draft teks dan foto berada di memori tab
dan hilang saat reload atau aplikasi ditutup. Ini bukan sinkronisasi offline.

Pembaruan tidak memaksa reload atau aktivasi worker saat draft sedang terbuka.
Worker baru menunggu instance aplikasi lama ditutup. Setelah deploy, tutup seluruh
jendela aplikasi lalu buka lagi bila ingin memuat pembaruan. Sesi operator tetap
mengikuti aturan login yang ada, sehingga aplikasi dapat meminta login lagi.

Jika menggunakan Cloudflare Access, selesaikan login Access sebelum memasang.
Pastikan manifest, ikon, dan `/sw.js` bisa dimuat setelah autentikasi. Jangan
cache `/sw.js` atau manifest secara permanen melalui aturan Cloudflare.

Pengujian PWA pada container lokal yang berjalan:

```sh
npm run test:pwa
```

Tes memeriksa manifest, ukuran ikon, installability Chromium, worker aktif,
halaman offline, pemulihan koneksi, dan bahwa API tidak disimpan dalam cache.
Instalasi pada perangkat Android/iPhone fisik tetap perlu diuji lewat domain Anda.

Referensi: [MDN installable PWA](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Guides/Making_PWAs_installable)
dan [panduan Apple](https://support.apple.com/guide/iphone/open-as-web-app-iphea86e5236/ios).
