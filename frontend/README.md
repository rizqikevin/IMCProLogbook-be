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
9. Pada pesan Anda, tekan **Hapus pesan**, lalu konfirmasi **Hapus pesan** dalam dialog. Penghapusan permanen mencakup seluruh foto pesan. **Batal** atau Escape menutup dialog dan mengembalikan fokus; saat penghapusan berlangsung, kontrol dan penutupan dialog dinonaktifkan. Jika gagal, pesan dan dialog tetap ada; gunakan **Coba hapus lagi**. Setelah sukses, baris pesan hilang tanpa menghapus draft teks atau foto.
10. Pada pesan Anda, tekan **Edit**, ubah **Teks pesan**, lalu **Simpan perubahan**. **Batal** atau Escape membatalkan perubahan tanpa mengubah draft kiriman. Foto pesan tetap sama. Pesan menampilkan **Diedit** setelah perubahan teks tersimpan, termasuk setelah reload; menyimpan teks yang sama tidak menandainya. Tidak ada batas waktu edit. Teks boleh dikosongkan hanya jika pesan memiliki foto.

**Ambil foto dari kamera** membuka pratinjau dengan tombol **Ambil foto** tetap
terjangkau. Ambil beberapa foto tanpa membuka ulang kamera. Jumlah dan thumbnail
mencakup foto galeri yang sudah dipilih, dengan batas bersama 20 foto. Tekan thumbnail
untuk memeriksa seluruh foto; **Hapus foto**, lalu potret lagi untuk menggantinya.
**Selesai** kembali ke chat. Tutup atau Escape juga mempertahankan foto yang selesai
diambil. Foto belum diunggah sampai **Kirim pesan** ditekan.

Foto percakapan baru diunduh saat masuk area terlihat, dengan margin 300 px,
atau saat tombol foto menerima fokus/ditekan. Placeholder menjaga ruang foto
sebelum unduhan dimulai. Browser tanpa `IntersectionObserver` memuat foto langsung.
Jika unduhan gagal, tekan **Coba lagi** pada foto. Membuka penampil mengunduh foto
yang dipilih saja; foto sebelumnya/berikutnya tidak diunduh lebih awal. Blob URL
dimiliki masing-masing tampilan dan dilepas saat foto berganti atau tampilan ditutup.

Smoke Chromium dengan API mock (20 foto): pembukaan percakapan meminta 2 foto
terakhir; scroll ke awal menambah 2 foto pertama, bukan seluruh 20 foto. Respons
500 pada salah satu foto pulih setelah **Coba lagi**. Penampil menampilkan foto
terpilih dan dapat ditutup dengan Escape. Smoke ini tidak menggantikan E2E backend
PostgreSQL.

Penampil foto percakapan dan arsip menyediakan **Perbesar**, **Perkecil**, dan
**Atur ulang** (100–400% dari ukuran pas layar). Setelah diperbesar, seret foto
dengan mouse atau satu jari; cubit dengan dua jari untuk mengubah ukuran.
Fokuskan foto lalu gunakan Shift + tombol panah untuk menggeser lewat keyboard.
Tombol panah kiri/kanan tanpa Shift tetap membuka foto sebelumnya/berikutnya;
Escape menutup penampil. Ukuran dan posisi kembali ke awal saat foto berganti.

Tombol **Tandai foto** (lingkaran di sebelah rotasi) mengaktifkan marker titik merah.
Klik atau ketuk beberapa posisi untuk menyimpannya di server. Marker mengikuti zoom,
pan, rotasi, dan ukuran layar; muat ulang tetap menampilkan titik tersimpan.
Pengunggah foto dan admin dapat menambah marker; pengguna lain hanya melihat.
Saat mode marker aktif, panah menggerakkan kursor sesuai arah layar, termasuk setelah
foto diputar; Enter menyimpan. Seret tetap menggeser foto tanpa menambah titik.
Gagal simpan ditampilkan, bukan dianggap berhasil.


Smoke Chromium memakai harness komponen bersama dengan dua gambar uji:
desktop 1280×900 memperbesar hingga 200% dan menyeret 120×90 px; viewport
mobile 390×844 menggeser lewat sentuhan dan mencubit dari 150% ke 300%, tanpa
overflow horizontal. Batas 400%, tombol atur ulang, reset saat navigasi,
Shift + panah, navigasi panah, dan Escape diperiksa. Tidak ada error runtime
pada smoke akhir. Harness ini tidak menguji API backend atau perangkat fisik.

**Ganti kamera** beralih depan/belakang. Jika izin ditolak, izinkan kamera melalui
pengaturan peramban lalu **Coba lagi**, atau gunakan **Pilih dari galeri**. Menutup
kamera atau berganti kamera menghentikan stream, termasuk izin yang baru selesai
setelah dialog ditutup. Pratinjau tidak dipotong. Kamera meminta resolusi ideal
3840×2160; peramban memilih resolusi yang tersedia bila perangkat tidak mendukungnya.
Foto memakai dimensi asli stream dan JPEG kualitas 94%, tanpa pengecilan atau
pembesaran paksa. Ketajaman tetap bergantung pada fokus, cahaya, dan kemampuan kamera.

Teks ditampilkan sebagai teks biasa, bukan HTML atau Markdown. Baris baru, spasi,
dan tab tetap disimpan. Satu kiriman menjadi satu pesan; tidak ada parser WhatsApp,
impor, atau sinkronisasi realtime.

Arsip lama tampil sebagai arsip foto dengan tanggal dan shift aslinya, tanpa teks
buatan. Tautan `/logbooks/:id` dan navigasi shift lama tetap dapat dibaca. Form
pembuatan arsip lama dan penambahan halaman tidak lagi menjadi route frontend.
Admin tetap dapat menghapus arsip lama melalui detail; backend memverifikasi izin.
Tombol hapus pada percakapan hanya tersedia untuk pesan native milik akun aktif,
bukan pesan pengguna lain atau arsip lama. Admin juga tidak dapat menghapus pesan
native pengguna lain; izin arsip lama tetap terpisah.
Edit juga hanya tersedia untuk pesan native milik akun aktif. Backend menolak edit
pesan pengguna lain, termasuk oleh admin; arsip lama tidak dapat diedit melalui chat.

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

`DELETE /api/v1/machines/{machineID}/messages/{messageID}` memakai Bearer token
dan hanya mengizinkan pengirim menghapus pesan native pada mesin aktif di path.
Role admin tidak memberi pengecualian. Respons `204` tidak memiliki body;
`400` berarti format ID tidak valid, `401` berarti autentikasi tidak valid, dan
`403` berarti pengguna bukan pengirim. API mengembalikan `404` jika pesan tidak
ada, sudah dihapus, berada pada mesin lain, mesin tidak ada atau tidak aktif,
atau ID merujuk arsip legacy. Penghapusan berulang juga menghasilkan `404`.
Setelah sukses, pesan tidak muncul di timeline dan foto langsung tidak dapat
diakses melalui API. Penghapusan file storage masuk antrean cleanup durable
untuk diproses background. Fitur ini tidak memerlukan migrasi database.

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
Kasus kamera mencakup beberapa jepretan, dimensi asli, pemilihan galeri, pemeriksaan
dan penghapusan, batas 20 foto, selesai/tutup/buka ulang, penolakan izin, pelepasan
stream terlambat, serta tombol rana dalam viewport portrait dan landscape.

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
