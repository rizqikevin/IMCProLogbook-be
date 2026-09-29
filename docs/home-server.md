# Docker langsung untuk Mac dan Linux home server

Jalankan dari root repository:

```sh
docker compose up -d --build
docker compose ps
```

Tidak perlu membuat `.env`. Konfigurasi aplikasi ditulis langsung dalam
`docker-compose.yml`: frontend React ikut dibuild, API Go, migration, PostgreSQL,
dan penyimpanan foto lokal. Tidak memakai Nginx, S3, atau MinIO.

Buka `http://localhost:18472`. Foto disimpan dalam Docker volume `uploads` dan
metadata dalam `db_data`. Rebuild/restart tidak menghapus kedua volume tersebut.
Jangan jalankan `docker compose down -v` jika ingin mempertahankan data.

Frontend mendengarkan port 18472. Backend memakai port 18473 di jaringan Docker
(`app:18473`), tanpa port publik; frontend meneruskan `/api` ke backend tersebut.

## Konfigurasi sementara

Database memakai username/password internal `logbook` yang tertulis di Compose.
Database dan API tidak memublikasikan port ke host. Hanya frontend yang tersedia
di `127.0.0.1:18472`. Akun aplikasi tetap dibuat sendiri, bukan memakai password
PostgreSQL. Konfigurasi `.env` lama seperti `STORAGE_DRIVER=s3` tidak diteruskan
ke container; Compose menetapkan `STORAGE_DRIVER=local` secara langsung.

Jika database lama menggunakan password berbeda, sesuaikan password di tiga tempat
pada Compose: service `db`, URL database `migrate`, dan URL database `app`.
Mengganti konfigurasi tidak mengganti password role PostgreSQL dalam volume lama.
Jangan menghapus volume untuk mengatasi ketidaksesuaian password.

Nama project mengikuti folder seperti sebelumnya, sehingga volume Mac tetap sama.
Untuk instalasi home server lama dengan nama project `machine-logbook`, gunakan:

```sh
docker compose -p machine-logbook up -d --build
```

`docker compose -f compose.home.yml up -d --build` juga tersedia sebagai wrapper
kompatibilitas dengan nama project lama. Gunakan nama project yang sama setiap kali.
Jika `.env` lama berisi `COMPOSE_PROJECT_NAME`, Compose sendiri masih dapat membaca
pengaturan tersebut; pilih project yang benar dengan `-p` untuk mempertahankan data.
Foto yang sudah berada di S3 tidak otomatis dipindahkan ke volume lokal.

## Akun admin dan operator

```sh
docker compose exec app /bin/logbook user create --username admin --name "Administrator" --role admin
docker compose exec app /bin/logbook user create --username operator1 --name "Operator 1" --role operator
```

Password diminta secara interaktif, minimal 12 karakter. Tambahkan `-p machine-logbook`
pada semua perintah jika memakai nama project tersebut.

## Cloudflare Tunnel

Gunakan connector Cloudflare yang sudah berjalan di home server. Connector tidak
lagi dibundel dalam Compose aplikasi sehingga token tidak perlu dimasukkan ke file
project atau GitHub.

- Connector di host Linux: arahkan service ke `http://localhost:18472`.
- Connector Docker: sambungkan ke network `<nama-project>_tunnel`, lalu arahkan ke
  `http://web:18472`. Pertahankan sambungan network tersebut dalam Compose connector.

Satu hostname tanpa pembatasan path melayani frontend serta `/api`. Pertahankan
HTTP Host Header publik. Login operator tetap diperlukan meskipun memakai Access.
Gunakan domain HTTPS untuk kamera dan instalasi PWA pada ponsel.

## Development React

Setelah stack berjalan, dari direktori `frontend`:

```sh
npm ci
npm run dev
```

Vite meneruskan API ke port 18472. Environment frontend hanya diperlukan jika ingin
mengubah alamat development proxy, bukan untuk menjalankan Docker Compose.

## Backup dan update

```sh
docker compose exec -T db pg_dump -U logbook -d logbook -Fc > logbook.dump
```

Backup volume foto `uploads` juga. Hentikan penulisan aplikasi saat mengambil backup
agar database dan foto konsisten, lalu simpan salinan di luar server. Data Mac tidak
otomatis disalin ke Linux.

Setelah mengambil source terbaru, jalankan lagi `docker compose up -d --build`.
Batas request 100.000.000 byte dan total foto frontend 95.000.000 byte. Batas upload
serta timeout Cloudflare tetap berlaku; koneksi tunnel di home server perlu diuji
pada server Anda.
