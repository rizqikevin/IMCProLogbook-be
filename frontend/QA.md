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
