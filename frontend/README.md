# Machine Logbook Archive · React

Frontend React (JavaScript) + Vite, tersambung langsung ke REST API Go.

## Jalankan lokal

Jalankan backend di port 8080 terlebih dahulu. Dari direktori `frontend`:

```sh
npm ci
npm run dev
```

Buka `http://localhost:5173`, lalu masuk menggunakan akun operator atau admin backend.
Tidak ada akun/password default yang ditambahkan ke database aplikasi.

Vite meneruskan `/api` ke `http://127.0.0.1:8080`. Jika alamat backend berbeda,
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

1. Login dengan akun masing-masing operator.
2. Buka **Arsip baru**, pilih mesin, tanggal logbook, dan shift.
3. **Buka kamera** untuk mengambil beberapa foto tanpa menutup kamera; atau **Pilih foto**.
4. Periksa foto, hapus foto yang tidak diperlukan, atau ubah urutannya.
5. Tekan **Done · Simpan arsip**. Semua foto dikirim dalam satu request multipart.
6. Cari arsip dengan filter mesin, rentang tanggal, dan shift. Detail arsip menyediakan
   navigasi halaman, perbesar foto, unduh foto, dan tambah halaman.

Operator dapat membaca dan menambah arsip. Hanya admin yang mendapat tombol hapus;
backend tetap memverifikasi hak akses. Foto dibaca sebagai blob menggunakan Bearer token,
bukan tautan storage publik. Blob URL dan media track kamera dilepas setelah digunakan.

Kamera langsung membutuhkan HTTPS atau localhost. Untuk ponsel melalui alamat IP LAN
HTTP, gunakan **Pilih foto** atau **Gunakan kamera perangkat**, atau sediakan HTTPS.
Izin dan pergantian kamera mengikuti kemampuan browser/perangkat.

Foto kamera diambil sebagai JPEG kualitas 94%, dengan sisi terpanjang maksimal 2560px.
Foto pilihan dari perangkat dipertahankan tanpa kompresi ulang. HEIC perlu dikonversi
menjadi JPEG sebelum dipilih.

## Perilaku penyimpanan dan retry

- Foto sebelum Done hanya berada di memori tab. Ada peringatan saat meninggalkan
  halaman; tidak ada sinkronisasi offline atau penyimpanan draft permanen.
- Session token disimpan di `sessionStorage` (atau memori jika storage tidak tersedia).
  Password tidak disimpan. Sesi dicek ke backend setelah refresh dan dibersihkan saat 401.
- Upload tidak diulang otomatis. Jika koneksi terputus, **Periksa hasil unggahan**
  mencocokkan SHA-256 foto dengan arsip. Append yang hasilnya belum pasti tetap diblokir
  agar tidak menggandakan halaman; operator dapat membuka arsip untuk memeriksa.
- Batas frontend mengikuti default backend: 20 foto per request, 100 halaman per arsip,
  10 MiB/foto, 40 megapiksel, dan total foto maksimal 95.000.000 byte agar tersedia ruang multipart.
  Jika limit backend diubah, sesuaikan `src/lib/format.js` dan konfigurasi proxy.

## Build dan Docker

```sh
npm run build
npm run preview
```

`preview` hanya melihat hasil build dan tidak menyediakan proxy development. Gunakan
alamat API yang sesuai saat build, atau jalankan Docker berikut untuk integrasi API.

Dari direktori backend:

```sh
docker compose --profile web up --build -d
```

Frontend ada di `http://localhost:3000`. Server Go meneruskan `/api` ke service `app` dan
menangani deep link React Router. Tambahkan TLS pada ingress untuk penggunaan production
dan kamera ponsel. `WEB_PORT` dapat mengganti port lokal frontend.

Untuk Linux home server dengan Cloudflare Tunnel tanpa Nginx, ikuti
[panduan home server](../docs/home-server.md) dan gunakan `compose.home.yml`.

## Struktur

```text
src/
  auth.jsx           sesi dan proteksi route
  hooks.js           pembacaan master data, arsip, dan foto
  lib/api.js         fetch, Bearer token, error, multipart + progress
  lib/format.js      tanggal dan validasi pilihan foto
  components/        layout, dialog, kamera
  pages/             Login, Archives, Capture, Book
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
gunakan database production. Pengujian mencakup login, capture berulang dengan kamera
simulasi, upload batch, urutan halaman, penambahan halaman, akses admin, pencarian,
network error, sesi berakhir, keyboard, kontras axe, dan overflow desktop/ponsel.

Desain disepakati dalam `DESIGN.md`. Kamera fisik dan perilaku izin Safari/iOS tetap
perlu diuji di perangkat operator sebelum deployment.

## Install di ponsel (PWA)

Buka domain HTTPS aplikasi. Di Android Chrome, pilih menu **Install app / Tambahkan
ke layar utama**. Di iPhone Safari, pilih **Bagikan → Tambahkan ke Layar Utama**,
aktifkan **Buka sebagai App** bila tersedia, lalu tambah.

Manifest, ikon Android/maskable, dan Apple touch icon disertakan dalam build.
Service worker hanya aktif pada build production, bukan Vite development.
Gunakan Docker untuk pengujian yang sama dengan deployment.

Aplikasi tetap membutuhkan internet untuk login, membaca arsip, dan mengunggah.
Cache PWA hanya menyimpan halaman offline beserta CSS-nya; API, foto, dan token
login tidak dimasukkan ke Cache Storage. Draft foto tetap berada di memori dan
hilang jika aplikasi ditutup. Ini bukan fitur sinkronisasi offline.

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
