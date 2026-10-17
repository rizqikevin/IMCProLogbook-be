# Docker langsung untuk Mac dan Linux home server

Jalankan dari root repository:

```sh
docker compose up -d --build
docker compose ps
```

Tanpa `.env`, Compose memakai penyimpanan foto lokal. Frontend React ikut dibuild,
bersama API Go, migration, dan PostgreSQL. Tidak memakai Nginx atau MinIO.
Compose membaca `STORAGE_DRIVER` dari environment atau `.env`, dengan default
`local`. Periksa `.env` lama sebelum menjalankan stack karena `STORAGE_DRIVER=s3`
sekarang diteruskan ke container.

Dalam mode lokal, foto disimpan dalam Docker volume `uploads` dan metadata dalam
`db_data`. Buka `http://localhost:18472`. Rebuild/restart tidak menghapus kedua volume.
Jangan jalankan `docker compose down -v` jika ingin mempertahankan data.

Frontend mendengarkan port 18472. Backend memakai port 18473 di jaringan Docker
(`app:18473`), tanpa port publik; frontend meneruskan `/api` ke backend tersebut.

## Konfigurasi sementara

Database memakai username/password internal `logbook` yang tertulis di Compose.
Database dan API tidak memublikasikan port ke host. Hanya frontend yang tersedia
di `127.0.0.1:18472`. Akun aplikasi tetap dibuat sendiri, bukan memakai password
PostgreSQL. Pengaturan storage di `.env` diteruskan ke container; pengaturan
aplikasi lainnya tetap mengikuti nilai inline pada Compose.

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

## Memindahkan foto lokal ke Cloudflare R2

Biarkan `STORAGE_DRIVER=local` sampai kredensial tersedia dan seluruh file lama
selesai disalin. `.env.example` memuat konfigurasi publik R2 berikut, tanpa rahasia:

```dotenv
STORAGE_DRIVER=local
S3_BUCKET=imcp-logbook
AWS_REGION=auto
S3_ENDPOINT=https://cc1cc266785ec408f8915028bde01b99.r2.cloudflarestorage.com
S3_PATH_STYLE=true
S3_KEY_PREFIX=archieve/
AWS_ACCESS_KEY_ID=
AWS_SECRET_ACCESS_KEY=
AWS_SESSION_TOKEN=
```

Endpoint hanya berisi alamat akun, tanpa `/imcp-logbook`. Ejaan prefix memang
`archieve/`. Isi access key dan secret key R2 dengan izin baca/tulis objek pada
bucket tersebut. Isi session token hanya jika kredensial sementara memerlukannya.
Simpan rahasia di `.env` server, jangan commit atau membagikan output konfigurasi
yang memuat rahasia. Jika `.env` sudah ada, gabungkan pengaturan storage tanpa
menimpa konfigurasi lain atau nama project.

Prefix hanya menerima huruf ASCII, angka, `-`, `_`, dan titik dalam segmen yang
dipisahkan `/`. Segmen tidak boleh kosong atau berupa `.` maupun `..`. Slash di
akhir bersifat opsional dan dinormalisasi oleh backend.

Lakukan perpindahan saat aplikasi tidak menerima penulisan:

1. Ambil backup database dan volume `uploads`. Gunakan nama project yang sama
   pada semua perintah, termasuk `-p machine-logbook` jika diperlukan.
2. Hentikan frontend dan API, lalu salin isi volume ke direktori sementara baru:

   ```sh
   docker compose stop web app
   mkdir -p ./uploads-r2-migration
   docker compose cp app:/app/data/uploads/. ./uploads-r2-migration/
   ```

3. Siapkan AWS CLI dengan kredensial R2 yang sama melalui environment shell atau
   profil kredensial lokal. AWS CLI tidak membaca `.env` Compose secara otomatis.
   Salin seluruh isi direktori ke prefix tujuan, tanpa menghapus objek tujuan:

   ```sh
   aws s3 sync ./uploads-r2-migration/ s3://imcp-logbook/archieve/ --region auto --endpoint-url https://cc1cc266785ec408f8915028bde01b99.r2.cloudflarestorage.com
   ```

   Pertahankan path relatif setiap file. Sebagai contoh, key logis
   `messages/550e8400-e29b-41d4-a716-446655440000/550e8400-e29b-41d4-a716-446655440001.jpg`
   harus menjadi nama objek yang sama dengan tambahan `archieve/` di depannya,
   tanpa segmen `uploads/`. Jangan mengubah key dalam database atau menambahkan
   prefix ke URL API. Backend menambahkan prefix saat mengakses R2.
4. Periksa hasil salinan terhadap file sumber, termasuk isi file, sebelum
   mengaktifkan R2. Jika kredensial atau salinan belum siap, jangan ubah driver.
5. Setelah salinan lengkap, batasi akses pengguna melalui connector Cloudflare
   selama pengujian. Ubah `.env` server menjadi `STORAGE_DRIVER=s3`, lalu buat
   ulang container agar pengaturan baru diterapkan:

   ```sh
   docker compose up -d --build
   ```

   Uji pembacaan foto lama dan upload foto baru melalui aplikasi sebelum membuka
   akses pengguna kembali melalui connector Cloudflare.

Mode S3 tidak otomatis membaca volume lokal ketika objek R2 hilang atau akses R2
gagal. Pergantian driver tidak menyalin file. Volume `uploads` tetap terpasang dan
tidak dihapus, tetapi upload baru dalam mode S3 hanya disimpan di R2. Jangan
kembali ke mode lokal tanpa menyalin perubahan R2 ke lokasi lokal yang sesuai.
API terhubung ke network `tunnel` untuk akses keluar ke R2; port API tetap tidak
dipublikasikan ke host.

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

## Chat per mesin

Setelah login, pilih mesin untuk membuka percakapan mesin tersebut. Ketik atau
tempel teks, lampirkan foto bila perlu, lalu kirim. Pesan boleh berisi teks saja,
foto saja, atau keduanya. Tanggal dan shift tidak perlu diisi. Aplikasi memakai
akun yang sedang login dan waktu server untuk mencatat pengirim dan waktu pesan.

Riwayat arsip tetap tersedia bersama foto aslinya. Arsip lama mempertahankan
tanggal dan shift sebagai keterangan arsip, bukan syarat untuk mengirim pesan baru.
Teks arsip lama tidak dibuat otomatis dari foto. Alur ini tidak mengimpor WhatsApp
dan tidak menyediakan pembaruan realtime atau penyuntingan pesan.

Deploy backend dan frontend bersamaan dengan perintah Compose di atas. Service
`migrate` menjalankan migrasi sebelum backend menerima request. Ambil backup
database dan volume foto sebelum pembaruan; jangan menghapus volume untuk
menerapkan perubahan alur chat.

## Backup dan update

```sh
docker compose exec -T db pg_dump -U logbook -d logbook -Fc > logbook.dump
```

Backup volume foto `uploads` juga jika memakai storage lokal. Untuk mode S3,
backup objek dalam `imcp-logbook/archieve/`; volume lokal tidak memuat upload baru
di R2. Hentikan penulisan aplikasi saat mengambil backup agar database dan foto
konsisten, lalu simpan salinan di luar server. Data Mac tidak otomatis disalin ke Linux.

Setelah mengambil source terbaru, jalankan lagi `docker compose up -d --build`.
Batas request 100.000.000 byte dan total foto frontend 95.000.000 byte. Batas upload
serta timeout Cloudflare tetap berlaku; koneksi tunnel di home server perlu diuji
pada server Anda.
