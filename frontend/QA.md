# Verifikasi frontend

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
