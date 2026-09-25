# Docker yang sama untuk Mac dan Linux home server

Kedua lingkungan memakai `docker-compose.yml`, Dockerfile yang sama, dan perintah
yang sama. Stack menjalankan React melalui server Go, API Go, migration, PostgreSQL,
dan volume foto lokal. Tidak memakai Nginx. Image dibuild mengikuti arsitektur host.

## Instalasi baru

Jalankan dari root repository dengan Docker Engine/Desktop dan Compose v2:

```sh
cp .env.example .env
chmod 600 .env
openssl rand -hex 32
```

Masukkan hasil generator ke `POSTGRES_PASSWORD` di `.env`. Password hex aman untuk
URL database. Lalu jalankan di Mac maupun Linux:

```sh
docker compose up -d --build
docker compose ps
```

Aplikasi tersedia di `http://localhost:3000`. Frontend, API, dan database dimulai
bersama; tidak perlu profile `web`. Database dan API hanya berada di jaringan
Docker. Hanya port frontend yang dipublikasikan ke loopback host.

`STORAGE_DRIVER=local` berlaku untuk development maupun production. Foto berada di
volume `uploads`, metadata di volume `db_data`. Restart/rebuild container tidak
menghapus data. Untuk external S3, set `STORAGE_DRIVER=s3` dan isi bucket,
region, endpoint HTTPS, serta kredensialnya. S3 tetap opsional.

## Memperbarui instalasi yang sudah ada

Jangan menimpa `.env` yang sudah berisi konfigurasi. Tambahkan `POSTGRES_PASSWORD`
sesuai password database saat ini. Mengubah variabel ini tidak mengubah password
role dalam volume PostgreSQL yang sudah terinisialisasi.

Pada Mac yang sudah memakai stack lama, nama project dan volume tetap mengikuti
folder yang sama; volume database serta foto tetap dipakai. Pada home server lama
yang memakai `compose.home.yml`, pertahankan `COMPOSE_PROJECT_NAME=machine-logbook`
di `.env` agar perintah baru mengakses volume lama. Salin pengaturan `.env.home`
yang diperlukan ke `.env`. Jika foto sebelumnya berada di S3, tetap set
`STORAGE_DRIVER=s3` beserta semua kredensialnya. Pergantian driver tidak memindahkan
foto secara otomatis.

`compose.home.yml` hanya menjadi wrapper kompatibilitas yang menyertakan file
utama dan mempertahankan nama project lama. Perintah lama dengan `--env-file
.env.home -f compose.home.yml` masih tersedia, tetapi pengaturan storage perlu
eksplisit seperti dijelaskan di atas. Gunakan satu cara menjalankan stack secara
konsisten, bukan dua project bersamaan.

Service MinIO development bawaan tidak lagi dijalankan oleh file utama. Jika
sebelumnya memakainya, pertahankan service storage tersebut secara terpisah;
volume MinIO lama tidak dihapus oleh perubahan ini.

## Akun admin dan operator

```sh
docker compose exec app /bin/logbook user create --username admin --name "Administrator" --role admin
docker compose exec app /bin/logbook user create --username operator1 --name "Operator 1" --role operator
```

Password diminta secara interaktif, minimal 12 karakter.

## Cloudflare Tunnel di home server

Jika cloudflared sudah berjalan sebagai service di host Linux, arahkan Published
application route domain Anda ke `http://localhost:3000` tanpa pembatasan path.
Browser mengakses domain HTTPS tersebut, termasuk API melalui `/api`.

Jika ingin menjalankan connector dari Compose ini, isi `TUNNEL_TOKEN` di `.env`:

```sh
docker compose --profile tunnel up -d
```

Service URL pada dashboard Cloudflare: `http://web:8080`. Untuk connector Docker
yang sudah ada, sambungkan ke network `<nama-project>_tunnel` milik stack ini
melalui konfigurasi external network connector. Lihat nama project dengan
`docker compose ls`. Jangan gunakan localhost untuk menghubungkan dua container.
Jika connector sebelumnya memakai `machine-logbook-tunnel`, ubah koneksi network
ke `machine-logbook_tunnel` pada instalasi dengan nama project `machine-logbook`.

Pertahankan HTTP Host Header publik. API memvalidasi same-origin sehingga domain
tunnel tidak perlu ditulis dalam allowlist CORS. Jika memakai frontend pada origin
berbeda, isi `CORS_ALLOWED_ORIGINS` dengan origin HTTPS yang tepat.

Cloudflare Access bersifat opsional sebagai gate tambahan; login operator aplikasi
tetap diperlukan. HTTPS domain diperlukan untuk kamera dan instalasi PWA dari
ponsel. Jangan cache `/api/*`, manifest, atau service worker secara permanen.

Panduan connector: [Cloudflare Tunnel](https://developers.cloudflare.com/tunnel/get-started/).

## Development React

Stack Docker yang sama dapat melayani API ketika menggunakan Vite:

```sh
cd frontend
npm ci
npm run dev
```

Vite meneruskan `/api` melalui `http://127.0.0.1:3000`. Untuk Go yang dijalankan
langsung di luar Docker, ubah `API_PROXY_TARGET` ke alamat proses Go tersebut.

## Backup dan update

```sh
docker compose exec -T db pg_dump -U logbook -d logbook -Fc > logbook.dump
```

Backup volume `uploads` juga, bukan hanya database. Hentikan penulisan aplikasi
saat mengambil backup database dan foto agar keduanya konsisten; simpan salinan di
luar server dan uji restore. Jangan menjalankan `down -v` karena menghapus volume.
Database/foto di Mac tidak otomatis disalin ke Linux; pindahkan dengan backup dan
restore bila diperlukan.

Setelah mengambil source terbaru, jalankan `docker compose up -d --build`.
Jika memakai connector Compose, jalankan dengan `--profile tunnel`.
Batas total foto frontend 95.000.000 byte; default request backend 100.000.000 byte.
Batas upload dan timeout Cloudflare tetap berlaku. Koneksi domain dan penyimpanan
pada home server Anda memerlukan verifikasi di server tersebut.
