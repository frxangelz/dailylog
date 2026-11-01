# dailylog

Logger Go yang sederhana, tanpa dependensi eksternal, dan **aman untuk banyak goroutine**.

- 📁 File log **harian** `yyyy-mm-dd.log`, semuanya di **satu folder khusus**
- 📍 Mencatat **nama file + nomor baris** pemanggil (mis. `main.go:42`, tanpa path)
- 🧹 **Hapus otomatis** file log yang lebih tua dari N hari (default 30, bisa diatur)
  — dijalankan sekali saat start, lalu berkala lewat goroutine (interval bisa diatur)
- 🖥️ Bisa print ke **console** dan/atau tulis ke **file** (masing-masing bisa di-enable/disable)
- 🎚️ Level **DEBUG**, **INFO**, **ERROR** — masing-masing bisa di-enable/disable, juga saat runtime
- 🔒 Thread-safe (sudah diuji dengan `go test -race`)

## Instalasi

```bash
go get github.com/frxangelz/dailylog
```

> Butuh Go 1.21 atau lebih baru.

## Mulai cepat

```go
package main

import (
    "errors"

    "github.com/frxangelz/dailylog"
)

func main() {
    if err := dailylog.Init(dailylog.WithDir("logs")); err != nil {
        panic(err)
    }
    defer dailylog.Close() // tutup file & hentikan goroutine cleanup

    dailylog.Info("server mulai di port", 8080)
    dailylog.Debugf("user=%s", "budi")
    dailylog.Error("gagal konek:", errors.New("timeout"))
}
```

Output di console **dan** di file `logs/2026-10-28.log`:

```
2026-10-28 15:04:05.123 [INFO ] main.go:14: server mulai di port 8080
2026-10-28 15:04:05.123 [DEBUG] main.go:15: user=budi
2026-10-28 15:04:05.124 [ERROR] main.go:16: gagal konek: timeout
```

Format baris: `tanggal jam.milidetik [LEVEL] file.go:baris: pesan`

Contoh lengkap ada di [`example/main.go`](example/main.go):

```bash
go run ./example
```

## Daftar option

Semua option bersifat opsional. Jika tidak diberikan, nilai default dipakai.

| Option | Fungsi | Default |
|---|---|---|
| `WithDir(dir)` | Folder tempat semua file log disimpan (dibuat otomatis) | `"logs"` |
| `WithFileEnabled(bool)` | Tulis log ke file | `true` |
| `WithConsoleEnabled(bool)` | Print log ke console | `true` |
| `WithDebug(bool)` | Aktifkan level DEBUG | `true` |
| `WithInfo(bool)` | Aktifkan level INFO | `true` |
| `WithError(bool)` | Aktifkan level ERROR | `true` |
| `WithMaxAgeDays(n)` | Umur maksimal file log (hari). `n <= 0` = tidak pernah dihapus | `30` |
| `WithCleanupInterval(d)` | Jeda goroutine cleanup. `d <= 0` = hanya cleanup saat start | `24h` |
| `WithColor(bool)` | Warna ANSI pada tag level di console (file tidak pernah berwarna) | `false` |
| `WithCallerSkip(n)` | Lewati `n` stack frame tambahan (untuk fungsi pembungkus) | `0` |
| `WithConsoleWriters(out, err)` | Ganti tujuan console (DEBUG/INFO → `out`, ERROR → `err`) | `os.Stdout`, `os.Stderr` |

## Dua cara pemakaian

### 1. Logger global (paling praktis)

```go
dailylog.Init(dailylog.WithDir("logs"))   // opsional; tanpa Init pakai default
defer dailylog.Close()

dailylog.Info("halo")
dailylog.Infof("jumlah=%d", 3)
dailylog.Debug(...)  / dailylog.Debugf(...)
dailylog.Error(...)  / dailylog.Errorf(...)
```

Jika `Init` tidak dipanggil, logger dibuat otomatis dengan konfigurasi default
saat pertama kali dipakai.

### 2. Instance sendiri

```go
l, err := dailylog.New(
    dailylog.WithDir("logs/payment"),
    dailylog.WithConsoleEnabled(false),
)
if err != nil { ... }
defer l.Close()

l.Info("pembayaran diterima")
```

Berguna jika Anda ingin beberapa logger dengan folder/pengaturan berbeda.
> Jangan arahkan dua logger ke folder yang **sama** dengan `MaxAgeDays` berbeda,
> karena yang lebih ketat akan menghapus file milik yang lain.

## Mengubah setelan saat runtime

Semua aman dipanggil dari goroutine mana pun, kapan saja:

```go
dailylog.SetLevelEnabled(dailylog.LevelDebug, false) // matikan DEBUG
dailylog.SetConsoleEnabled(false)                    // matikan console
dailylog.SetFileEnabled(true)                        // nyalakan file

// pada instance:
l.SetMaxAgeDays(7)
n, err := l.Cleanup() // jalankan pembersihan sekarang; n = jumlah file terhapus
```

## Cara kerja

**File harian.** Setiap log ditulis ke `<dir>/<yyyy-mm-dd>.log` menurut jam
lokal. Saat tanggal berganti (lewat tengah malam), logger otomatis menutup file
lama dan membuka file baru — tanpa perlu restart. Jika aplikasi restart di hari
yang sama, log **ditambahkan** (append) ke file hari itu, bukan ditimpa.

**Hapus otomatis.** File dianggap kedaluwarsa jika *tanggal pada nama file*
lebih lama dari `MaxAgeDays` hari. Dengan `MaxAgeDays = 30`, log 30 hari lalu
masih disimpan, log 31 hari lalu dihapus.
Pembersihan dijalankan:
1. sekali, secara sinkron, saat `New`/`Init`;
2. lalu oleh satu goroutine setiap `CleanupInterval`, sampai `Close()` dipanggil.

**Keamanan penghapusan.** Hanya file yang namanya persis berpola
`yyyy-mm-dd.log` yang bisa dihapus. File lain di folder yang sama
(`catatan.txt`, `app-2025-01-01.log`, subfolder, dst.) tidak pernah disentuh.

**Thread-safety.** Penulisan (file + console) dilindungi satu mutex sehingga
baris log tidak pernah tercampur antar goroutine. Flag enable/disable memakai
`sync/atomic`, jadi pengecekan level murah dan tidak memblokir. Level yang
dimatikan tidak memformat string sama sekali.

**Nama file & nomor baris.** Diambil dengan `runtime.Caller`, lalu dipotong
menjadi nama file saja (`main.go:42`). Jika Anda membungkus logger ini dalam
fungsi helper sendiri, pakai `WithCallerSkip(1)` agar yang tercatat adalah
pemanggil helper Anda.

**Console.** DEBUG dan INFO ke `stdout`, ERROR ke `stderr`.

**Error saat menulis.** Jika file gagal dibuka/ditulis (disk penuh, izin, dll.),
logger tidak panik dan tidak mengganggu aplikasi — pesan kegagalan dicetak ke
`stderr`, dan log ke console tetap berjalan.

## Menjalankan test

```bash
go test -race ./...
```

## Publish ke GitHub

1. Buat repo **public** baru di GitHub, mis. `dailylog`.
2. Ganti placeholder `USERNAME` dengan username/organisasi GitHub Anda:

   ```bash
   # Linux
   grep -rl USERNAME . | xargs sed -i 's/USERNAME/usernameanda/g'
   # macOS
   grep -rl USERNAME . | xargs sed -i '' 's/USERNAME/usernameanda/g'
   ```

   Hasilnya `module github.com/frxangelz/dailylog` di `go.mod`, dan juga
   di import pada `example/main.go` serta README ini.
3. Pastikan semuanya beres, lalu push:

   ```bash
   gofmt -l .          # harus kosong
   go vet ./...
   go test -race ./...

   git init
   git add .
   git commit -m "feat: initial release"
   git branch -M main
   git remote add origin https://github.com/frxangelz/dailylog.git
   git push -u origin main
   ```
4. Beri versi dengan tag (Go memakai semantic versioning):

   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```
5. Selesai! Di project lain:

   ```bash
   go get github.com/frxangelz/dailylog@v1.0.0
   ```

   ```go
   import "github.com/frxangelz/dailylog"
   ```

   Dokumentasi otomatis muncul di <https://pkg.go.dev/github.com/frxangelz/dailylog>
   (kadang perlu sekali membuka halaman itu atau menjalankan `go get` agar terindeks).

## Lisensi

MIT — lihat [LICENSE](LICENSE).
