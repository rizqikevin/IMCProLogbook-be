# Verifikasi frontend

## Pencarian teks per mesin · 11 Oktober 2026

- Backend integration PostgreSQL dengan race detector dan `go vet ./...` lulus.
- Suite browser desktop/Pixel 7: 16 pengujian lulus, termasuk pencarian seluruh
  riwayat, pagination hasil, isolasi mesin, karakter literal `%`/`_`, perbedaan
  huruf besar/kecil, reset, reload, Back/Forward, hasil kosong, dan retry.
- Pengiriman saat pencarian aktif tidak menyisipkan pesan yang tidak cocok;
  draft dan foto tetap ada saat query berubah. Respons usang tidak mengganti hasil.
- Smoke browser nyata: `BEARING` menemukan `Pelumasan bearing selesai`.
  Pada viewport 390 × 844, ukuran dokumen tetap 390 × 844; screenshot diambil.
- Hard gate PASS: axe dan overflow pada pencarian tercakup E2E; kontrol pencarian,
  reset, Escape, state loading/error/kosong berjalan.
- Purpose gate PASS: pencarian di header membatasi konteks pada mesin aktif;
  composer tetap tersedia dan query tersimpan pada URL.
- Liveliness PASS: arah navy dan ENERGY 1 / RHYTHM 2 / MOTION 1 tetap dipakai.
- Craftsmanship PASS: pencarian memakai API dan PostgreSQL nyata, bukan filter
  halaman yang sudah dimuat. Dokumen API dan petunjuk pengguna diperbarui.
- Batas: belum diuji pada perangkat fisik; screenshot tidak dinilai secara visual.

## Layout aplikasi chat · 11 Oktober 2026

- `npm test`: 13 pengujian lulus; `npm run build`: lulus.
- Playwright dengan backend Go dan PostgreSQL terisolasi: 12 pengujian lulus
  pada desktop dan Pixel 7. Kamera menangkap frame video uji, mengirim foto,
  lalu foto yang tersimpan dibaca kembali setelah reload.
- Header mesin dan composer tetap dalam viewport saat riwayat panjang digulir.
  Browser langsung pada 390 × 844 menunjukkan ukuran dokumen 390 × 844;
  composer berakhir tepat pada koordinat bawah 844. Pesan multiline tetap ada
  setelah reload. Bubble milik sendiri menampilkan Anda dan pemisah tanggal.
- Hard gate PASS: pengiriman teks/foto/kamera, galeri, hapus lampiran, dialog foto,
  Escape, state kosong/loading/error, retry, draft antarmesin, axe dan overflow
  tercakup suite yang lulus.
- Purpose gate PASS: header ringkas menjaga konteks mesin, bubble membedakan
  pengirim, dan composer bawah menjaga tindakan kirim tersedia; arah di DESIGN.md.
- Liveliness PASS: ENERGY 1 / RHYTHM 2 / MOTION 1, identitas navy IMCPro,
  foto mesin yang sudah ada, tanpa status online atau tanda baca fiktif.
- Craftsmanship PASS: interaksi memakai backend nyata; sesi pengujian digunakan
  ulang per worker tanpa melemahkan pembatasan login production.
- Batas: keyboard dan kamera perangkat fisik belum diuji. Screenshot browser
  langsung mengalami timeout; bukti layout memakai geometri DOM dan E2E.

## Chat per mesin · 11 Oktober 2026

- Backend: `make test`, `make lint`, dan `make test-integration` dengan PostgreSQL
  terisolasi lulus. Regresi normalisasi CRLF ke LF lulus lewat
  `go test -race ./internal/service` setelah perubahan terakhir.
- Frontend: 13 unit test, build production, dan 8 E2E desktop/Pixel 7 lulus.
- Browser nyata: login, pilih mesin, kirim teks multiline, muat ulang, dan baca
  pesan tersimpan berhasil. Pada lebar 390 piksel, scrollWidth 390, tinggi header
  percakapan 77 piksel, tombol kirim 46 piksel, dan fokus textarea terlihat.
- E2E: teks, foto, teks dengan foto, hapus lampiran sebelum kirim, perbesar foto,
  tutup dialog dengan Escape, baca arsip foto lama, draft saat koneksi gagal,
  draft terpisah per mesin, pagination, retry, serta pindah mesin saat mengirim lulus.
- Hard gate PASS: axe WCAG A/AA dan pemeriksaan overflow lulus, termasuk lebar
  320, 768, dan 1024 piksel. State kosong/loading/error tercakup E2E.
- Purpose gate PASS: gaya navy dan foto mesin mengikuti `DESIGN.md`; foto header
  kecil membedakan konteks mesin tanpa menggeser percakapan.
- Liveliness PASS: ENERGY 1 / RHYTHM 2 / MOTION 1 dipertahankan; hierarki mesin,
  riwayat, dan composer memakai gaya aplikasi yang sudah ada.
- Craftsmanship PASS: alur utama memakai backend Go dan PostgreSQL nyata;
  kegagalan jaringan disimulasikan hanya dalam pengujian.

Screenshot browser diambil untuk bukti layout. Pemeriksaan visual berbasis gambar
tidak dinilai pada sesi ini; ukuran DOM, overflow, fokus, axe, dan interaksi diuji.
Belum diverifikasi pada perangkat fisik atau deployment production.
Draft hanya tersimpan selama tab aktif; reload, tutup tab, atau logout menghapusnya.
Baris CRLF dinormalisasi menjadi LF; spasi dan baris kosong tetap disimpan.

## Riwayat verifikasi sebelumnya

Pengujian terakhir: 23 September 2026.

Pembaruan container tanpa Nginx: server Go diperiksa dengan `go test -race`
dan `go vet`, mencakup deep link SPA, aset, header keamanan, dan penerusan host,
Authorization, query, serta body upload. Konfigurasi Compose home server divalidasi
tanpa memakai kredensial production. Lima unit test frontend juga lulus kembali.

- `npm test`: 5 pengujian lulus.
- `npm run build`: lulus.
- Playwright dengan backend Go dan PostgreSQL terisolasi: 4 pengujian lulus,
  mencakup desktop dan emulasi Pixel 7.
- Alur yang diperiksa: login salah/benar, filter arsip, pilih dan urutkan foto,
  peringatan meninggalkan draft, unggah batch, baca dan perbesar halaman,
  kamera berulang, tambah halaman, pemulihan respons unggahan terputus,
  pembatasan hapus untuk admin, logout, sesi kedaluwarsa, dan error jaringan.
- Pemeriksaan axe pada login, daftar, form, dan detail tidak menemukan pelanggaran.
  Pemeriksaan overflow juga mencakup lebar 320, 768, dan 1024 piksel.

## Tinjauan desain antislop

- Hard gate: halaman utama berjalan, state kosong/loading/error tersedia,
  fokus keyboard dan dialog diuji; tidak ada statistik atau pengguna fiktif
  di aplikasi. Data dan foto pengujian hanya berada dalam lingkungan test.
- Purpose gate: alasan warna, tipografi, susunan daftar, dan kontrol tercatat
  di `DESIGN.md`. Ikon digunakan untuk tindakan kamera, file, dan navigasi.
- Liveliness: ENERGY 1 / RHYTHM 2 / MOTION 1 mengikuti arah industrial terang;
  aksen hijau menandai tindakan utama dan mesin terpilih.
- Craftsmanship: screenshot desktop dan ponsel diperiksa; interaksi utama
  diverifikasi melalui browser dengan API nyata, termasuk kegagalan jaringan.

## Batas verifikasi

Kamera menggunakan perangkat simulasi Chromium. Kamera fisik, pergantian lensa,
izin Safari/iOS, serta deployment HTTPS perlu diperiksa pada perangkat operator.
Pengujian ini tidak menyatakan seluruh kombinasi browser dan perangkat didukung.

## PWA · 24 September 2026

Build production dan container tanpa Nginx berhasil. `npm run test:pwa` lulus
terhadap container lokal: Chromium melaporkan nol installability errors, ikon
sesuai dimensi manifest, service worker aktif, navigasi offline menampilkan fallback,
dan koneksi yang pulih kembali menuju login. Cache Storage hanya berisi
`offline.html` dan `offline.css` setelah request API. Lima unit test frontend serta
pengujian server Go juga lulus.

Ikon memakai bentuk buku sederhana dari SVG lokal dengan warna aplikasi, untuk
identitas launcher. Tidak ada perubahan alur pengambilan foto. Instalasi Android,
iOS, dan alur Cloudflare Access pada domain production belum diuji di perangkat fisik.

## IMCPro UI refresh

Logo asli pengguna dipasang tanpa modifikasi pada login dan header. Navy dari logo
menjadi warna tindakan utama; filter, identitas logbook, dan pilihan foto dibedakan
melalui permukaan dan jarak. Screenshot login desktop/ponsel, daftar desktop, dan
capture ponsel diperiksa. Build production, 5 unit test, dan 4 E2E desktop/ponsel
lulus; E2E mencakup axe dan overflow hingga lebar 320 piksel. Gate antislop mengikuti
arah dalam DESIGN.md, tanpa data tambahan atau perubahan alur upload.

## Machine photos · 30 September 2026

All six machine cards load their supplied photos and filter by the selected machine.
Six desktop/mobile E2E tests passed, including upload workflows, axe, and overflow.
After correcting selected-card caption contrast, both gallery tests passed again
at widths 320, 390, 768, and 1440. Screenshots were reviewed and the Docker frontend
rebuilt. Photos use contain-fit without cropping; coating machines share one photo.

## Machine-first archive flow · 30 September 2026

Seven unit tests and eight desktop/mobile E2E tests passed. Coverage includes
machine selection before archive access, locked machine on creation, previous/next
shift navigation into existing and empty slots, year/date rollover, prefilled
creation from an empty shift, upload/append/delete, accessibility and overflow.
The login test helper honors the server's Retry-After header when the expanded
suite reaches the existing login rate limit; production limits are unchanged.
Desktop chooser and empty-shift screenshots, plus mobile detail and empty-shift
screenshots, were inspected. Production build and Docker frontend rebuild passed;
the web container is healthy on localhost:18472.

## Input mesin berikutnya dan tanggal H-1 · 30 September 2026

Delapan unit test dan delapan E2E desktop/mobile lulus. Setelah unggah berhasil,
pengujian memilih Input mesin lain, memilih MS3, dan memeriksa mesin, shift yang
terbawa, tanggal H-1, serta form tanpa foto sebelumnya. Unit test mencakup H-1
saat pergantian tahun dan tahun kabisat. Tanggal eksplisit dari shift kosong tetap
diuji. Build Docker berhasil dan container frontend diperbarui.

- Hard Gate PASS: E2E menguji alur unggah, tombol lanjut, keyboard, axe, error,
  dan overflow; screenshot detail desktop/mobile diperiksa.
- Purpose Gate PASS: tombol lanjut berada di konfirmasi unggah untuk mempercepat
  input mesin berikutnya; alasan dan aturan tanggal tercatat di DESIGN.md.
- Liveliness PASS: ENERGY 1 / RHYTHM 2 / MOTION 1 tetap dipakai; navy, logo asli,
  foto mesin, dan jarak antarkontrol mengikuti desain aplikasi yang sudah ada.
- Craftsmanship PASS: navigasi memakai tautan nyata, form berikutnya kosong,
  tidak ada foto atau statistik tambahan; delapan E2E dan build production lulus.

## Penyempurnaan operator · 30 September 2026

Halaman awal tetap hanya pilihan mesin. Foto mesin ditambahkan di atas filter
arsip, input tanggal menggunakan default H-1, dan ringkasan simpan menampilkan
mesin, tanggal, serta shift yang benar, termasuk saat menambah halaman.

- Hard Gate PASS: delapan unit dan delapan E2E lulus; setelah penyederhanaan
  halaman awal, empat E2E pilihan mesin/tanggal desktop-mobile lulus kembali.
  Pemeriksaan mencakup axe dan overflow. Tombol tanggal cepat kemudian dihapus
  sesuai permintaan pengguna; field tanggal dan default H-1 tetap dipertahankan.
- Purpose Gate PASS: alasan penanda mesin dan ringkasan simpan tercatat di DESIGN.md;
  tidak ada tambahan langkah sebelum memilih mesin.
- Liveliness PASS: tetap ENERGY 1 / RHYTHM 2 / MOTION 1 dengan navy dan foto asli.
- Craftsmanship PASS: screenshot form mobile dan pilihan mesin diperiksa,
  build Docker final berhasil, alur unggah dan peringatan draft tetap teruji.

## Teks ringkas dan jarak mobile · 30 September 2026

- Hard Gate PASS: delapan unit test dan delapan E2E desktop/mobile lulus,
  termasuk unggah, urutkan foto, tambah halaman, draft, shift kosong, serta
  tanggal H-1. Axe dan pemeriksaan overflow juga lulus.
- Purpose Gate PASS: label tindakan disederhanakan dan judul berulang dihapus
  untuk mempercepat pembacaan; alasan tercatat di DESIGN.md.
- Liveliness PASS: ENERGY 1 / RHYTHM 2 / MOTION 1 tetap konsisten; navy, foto
  mesin, dan logo asli dipertahankan tanpa langkah atau pilihan baru.
- Craftsmanship PASS: screenshot login dan form mobile serta pilihan mesin
  desktop diperiksa. Build production Docker berhasil, container sehat.

## Thumbnail kamera · 30 September 2026

- Hard Gate PASS: dua E2E alur unggah desktop/mobile lulus, termasuk dua foto
  kamera dengan thumbnail yang selesai dimuat, axe, dan overflow.
- Purpose Gate PASS: thumbnail bernomor di bawah kamera membantu memeriksa foto
  yang sudah dipilih; deret dapat digeser dan mengikuti foto terbaru.
- Liveliness PASS: foto pengguna dan gaya navy yang ada dipakai tanpa animasi baru.
- Craftsmanship PASS: screenshot kamera mobile diperiksa; build Docker berhasil.
  Preview memakai blob URL milik form, tanpa unggahan sebelum Simpan logbook.
