# Continuous deployment VPS

Workflow `backend` menjalankan CI di GitHub-hosted Ubuntu. Job `deploy` hanya
berjalan sesudah seluruh job `test` sukses untuk push `main` pada repository asli.
Pull request tidak menjalankan job produksi. Checkout memakai SHA yang sama dengan
CI, bukan HEAD terbaru. Deployment memakai environment `production`, label runner
`imcpro-production`, concurrency tanpa pembatalan deployment aktif, dan host lock.
GitHub dapat mengganti run yang masih pending dengan run lebih baru; run aktif
selesai lebih dahulu.

## Persiapan host

Host: `robot@192.168.1.10`. Konfigurasi produksi tetap berada di
`/home/robot/IMCProLogbook-be/.env`, bukan checkout Actions. Jangan menyalin `.env`
ke GitHub atau mencetak hasil `docker compose config` ke log. Script menyimpan
snapshot konfigurasi yang mengandung rahasia di `.cd/releases` dengan umask 077.

Sebelum mengaktifkan runner:

1. Identifikasi project dari label container aktif, jangan menebak dari nama folder:
   `docker ps --format '{{.Names}} {{.Label "com.docker.compose.project"}}'`.
2. Buat `/home/robot/IMCProLogbook-be/.cd` mode 700 untuk pengguna deploy dan tulis
   nama project tersebut ke `.cd/project`. Pastikan pengguna deploy dapat membaca
   `.env` dan konfigurasi Compose lama, serta menulis `.cd`.
3. Pasang GitHub Actions runner resmi sebagai service Linux non-root khusus.
   Gunakan runner repository ini saja, label `imcpro-production`, tanpa kredensial
   GitHub permanen dalam file setup. Token registration hanya dipakai sekali.
   Runner memerlukan Bash, Git, curl, flock, Docker Engine dan Compose v2 dengan
   `up --wait`. Jangan menambahkan passwordless sudo umum. Akses Docker socket
   tetap setara root pada host; akun non-root bukan sandbox terhadap job jahat.
4. Service runner memakai `UMask=0077`. Jangan gunakan runner untuk PR, eksperimen,
   atau repository lain. Repository ini publik: batasi siapa boleh mengubah workflow
   dan siapa boleh menyetujui deployment. Label runner hanya routing, bukan batas
   keamanan. Reviewer harus memeriksa semua perubahan workflow sebelum approval.
   Jika runner group dengan pembatasan workflow tersedia pada organisasi, gunakan
   pembatasan hanya workflow deployment pada branch main. Runner repository personal
   tidak menyediakan isolasi workflow tersebut; workflow lain yang diubah oleh
   maintainer tepercaya masih dapat menjadwalkan job pada runner ini.

Host ini memakai service systemd user `imcpro-actions-runner.service` milik `robot`,
direktori `/home/robot/actions-runner-imcpro`, dengan linger yang sudah aktif.
Service enabled, restart otomatis, dan `UMask=0077`; runner memperbarui dirinya
otomatis. Tidak ada akun baru atau akses sudo tambahan. Akun `robot` sudah memiliki
akses Docker dan `.env`; ini bukan akun deploy terisolasi. Kelola service dengan
`systemctl --user status imcpro-actions-runner.service` sebagai `robot`.
Project produksi teridentifikasi dari label container: `imcprologbook-be`, disimpan
di `.cd/project` mode privat. Jangan mengganti nama project saat deploy.

## Approval

Environment `production` dikonfigurasi melalui API GitHub dengan required reviewer
`rizqikevin` dan deployment branch hanya `main`. Self-review diizinkan agar pemilik
repository dapat menyetujui push sendiri; ini manual gate, bukan two-person review.
Admin bypass masih tersedia. Plan akun tidak diekspos API, tetapi API menerima
required-reviewer protection pada repository publik ini. Periksa Settings >
Environments > production sebelum mengaktifkan runner. Jangan menghapus gate untuk
mengatasi job yang menunggu approval.

## Deploy dan recovery

`bash scripts/deploy.sh <full-tested-sha>` hanya dijalankan dari checkout SHA yang
sudah lolos CI. Script menolak host tanpa `.env`, `.cd/project`, atau tiga container
aktif unik `app`, `web`, `db` pada project tersebut.

Script menyimpan resolved Compose lama dan image ID app/web sebagai tag rollback,
membangun kedua image baru sebelum mengganti container, lalu menjalankan migration.
Database dan upload volume tidak dihapus, direcreate, atau dipindahkan. Setelah itu
hanya app/web diganti dengan `--no-deps --no-build`, Docker health checks ditunggu,
dan HTTP frontend diperiksa. Jika penggantian atau health check gagal, app/web
sebelumnya dipulihkan dengan konfigurasi dan image yang disimpan. Run tetap gagal
agar kegagalan deployment tidak tersembunyi.

Migration tidak di-rollback: perubahan schema wajib backward-compatible dengan
release sebelumnya. Backup database tetap tanggung jawab operasi sebelum perubahan
schema; rollback image bukan restore database. SIGKILL/power loss tidak dapat
menjalankan trap; recovery manual memakai snapshot release. Jangan menjalankan
`docker compose down -v`, prune volume, atau menghapus image rollback. Snapshot dan
image sengaja ditahan; pantau kapasitas disk dan hapus release lama hanya setelah
memastikan tidak dibutuhkan untuk recovery. `.cd` tidak boleh dicadangkan ke lokasi
publik karena memuat resolved secrets.

Recovery manual (pilih path release gagal yang tercetak di log dan project asli):

```sh
docker compose -p "$PROJECT" --env-file /home/robot/IMCProLogbook-be/.env \
  -f "$RELEASE/previous.yml" -f "$RELEASE/old-images.yml" \
  up -d --no-deps --no-build --wait --wait-timeout 180 app web
```

## Pemeriksaan perubahan CD

```sh
bash -n scripts/deploy.sh
python3 scripts/deploy_test.py
```

Test memakai mock command dan direktori sementara, bukan host produksi. Live smoke
harus memeriksa run CI, approval, status runner, SHA `.cd/deployed-sha`, health
app/web dan akses frontend. Jangan menyatakan setup live selesai sebelum semua
langkah tersebut terbukti.
