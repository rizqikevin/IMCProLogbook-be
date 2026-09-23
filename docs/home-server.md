# Linux home server dengan Cloudflare Tunnel

Stack ini tidak memakai Nginx. React dibuild menjadi file statis dan dilayani oleh
server Go kecil yang juga meneruskan `/api/` ke backend. Hanya frontend yang
tersedia di loopback host; PostgreSQL dan API tidak memublikasikan port.

```text
Browser HTTPS → Cloudflare / Access → cloudflared → web:8080 → app:8080 → PostgreSQL
                                                                  → S3 HTTPS
```

## 1. Siapkan konfigurasi

Gunakan Docker Engine dan Compose v2 pada Linux, lalu salin/clone repository ini.
Jalankan dari direktori root repository:

```sh
cp .env.home.example .env.home
chmod 600 .env.home
openssl rand -hex 32
```

Isi `.env.home` menggunakan editor di server:

- `POSTGRES_PASSWORD`: hasil generator hex di atas. Format hex aman untuk URL database.
- `PUBLIC_ORIGIN`: domain HTTPS sebenarnya, misalnya `https://logbook.perusahaan.com`,
  tanpa slash terakhir.
- `S3_BUCKET`, endpoint HTTPS, region, dan kredensial S3-compatible yang sudah tersedia.
  Contoh file memakai Cloudflare R2; untuk AWS S3 kosongkan endpoint, pilih region
  bucket, dan set `S3_PATH_STYLE=false`.
- `TUNNEL_TOKEN`: hanya jika memakai service tunnel opsional di Compose ini.

Bucket harus sudah dibuat dan privat. Backend production tetap mensyaratkan S3;
Cloudflare Tunnel sendiri tidak menyediakan penyimpanan foto. Kredensial bucket
memerlukan akses baca, tulis, dan hapus objek aplikasi. Jangan masukkan secret ke
variabel frontend `VITE_*`.

File ini memakai nama project `machine-logbook` dan volume database tersendiri.
Data stack development tidak otomatis berpindah. Jangan mengubah password di file
setelah database terinisialisasi tanpa mengubah password role PostgreSQL juga.

## 2. Jalankan aplikasi

```sh
docker compose --env-file .env.home -f compose.home.yml up -d --build
docker compose --env-file .env.home -f compose.home.yml ps
curl -f http://127.0.0.1:3000/health
```

Migration dijalankan sebelum API mulai. Buat akun pertama secara interaktif:

```sh
docker compose --env-file .env.home -f compose.home.yml exec app /bin/logbook user create --username admin --name "Administrator" --role admin
```

Gunakan `--role operator` untuk akun operator. Password diminta oleh CLI.

## 3. Hubungkan tunnel yang sudah ada

Pilih cara sesuai lokasi connector Anda. Satu hostname menangani frontend dan API;
path pada Published application route dikosongkan.

| Lokasi cloudflared | Service URL di Cloudflare |
| --- | --- |
| Service systemd pada host Linux yang sama | `http://localhost:3000` |
| Container dalam network `machine-logbook-tunnel` | `http://web:8080` |
| Service opsional Compose di bawah | `http://web:8080` |

Untuk connector Docker yang sudah ada, tambahkan network external
`machine-logbook-tunnel` ke Compose milik connector setelah stack ini dijalankan.
Sambungkan service cloudflared ke network tersebut. Contoh potongan konfigurasi:

```yaml
services:
  cloudflared:
    # Pertahankan image, command, token, dan network yang sudah ada.
    networks:
      - logbook
networks:
  logbook:
    external: true
    name: machine-logbook-tunnel
```

`localhost` di dalam container connector menunjuk container itu sendiri,
sehingga gunakan `web:8080` untuk cara Docker. Jangan mengganti HTTP Host Header
origin dengan nama service; pertahankan hostname publik browser.

Jika ingin connector khusus dari stack ini, isi token tunnel lalu jalankan:

```sh
docker compose --env-file .env.home -f compose.home.yml --profile tunnel up -d
```

Atur Published application route untuk domain `PUBLIC_ORIGIN` dengan service
`http://web:8080`. Token dibaca dari environment `TUNNEL_TOKEN`.
Lihat [panduan resmi tunnel](https://developers.cloudflare.com/tunnel/get-started/)
dan [parameter token](https://developers.cloudflare.com/tunnel/reference/run-parameters/).

Tambahkan aplikasi Cloudflare Access dan policy pengguna jika ingin gate Zero Trust
sebelum login operator. Login operator aplikasi tetap diperlukan; integrasi ini
belum melakukan SSO dari identitas Access. Jangan buat aturan cache yang menyimpan
respons `/api/*`; server mengirim `Cache-Control: no-store` untuk API.

Buka domain HTTPS lalu uji login, ambil beberapa foto, Done, dan baca arsip.
HTTPS pada domain publik mendukung penggunaan kamera browser. Tidak perlu membuka
port router untuk stack ini; koneksi tunnel dibuat keluar oleh cloudflared.

## Operasional

Frontend membatasi total foto satu batch ke 95.000.000 byte, dan backend home
server membatasi request ke 100.000.000 byte. Sesuaikan dengan batas upload yang
berlaku pada akun Cloudflare; jika batas zone lebih kecil, kecilkan batch foto.
Timeout di edge juga tetap berlaku pada koneksi lambat. Gunakan tombol pemeriksaan
hasil upload jika respons terputus, sebelum mencoba unggah ulang.

Untuk pembaruan aplikasi setelah mengambil source terbaru:

```sh
docker compose --env-file .env.home -f compose.home.yml up -d --build
docker compose --env-file .env.home -f compose.home.yml logs --tail=100 web app
```

Backup database dan objek S3 sebagai satu kumpulan arsip. Contoh dump database:

```sh
docker compose --env-file .env.home -f compose.home.yml exec -T db pg_dump -U logbook -d logbook -Fc > logbook.dump
```

Simpan backup di luar server dan uji restore sebelum mengandalkannya. Hindari
`down -v` karena menghapus volume database. Docker image memakai arsitektur host
saat build, termasuk Linux amd64 dan arm64. Pin tag/digest image yang sudah diuji
jika ingin pembaruan image sepenuhnya terkontrol.

Koneksi Cloudflare dan S3 di server Anda belum diuji oleh pengujian lokal repository;
memerlukan domain, token, dan bucket milik Anda.
