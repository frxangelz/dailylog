// Contoh pemakaian dailylog. Jalankan dari root repo:
//
//	go run ./example
package main

import (
	"errors"
	"sync"
	"time"

	"github.com/USERNAME/dailylog"
)

func main() {
	// 1) Inisialisasi logger global (semua option opsional).
	err := dailylog.Init(dailylog.DailyLogConfig{
		Dir:                "logs",
		FileEnabled:        true,
		ConsoleEnabled:     true,
		MaxAgeDays:         30,             // 0 = disable
		CleanupInterval:    12 * time.Hour, // 0 = disable
		FullTimeLayout:     false,
		FullFilenameLayout: false,
		CallerInfo:         true,
	})

	if err != nil {
		panic(err)
	}
	defer dailylog.Close() // tutup file & hentikan goroutine cleanup

	// 2) Pemakaian dasar. Perhatikan "main.go:NN" pada output.
	dailylog.Info("aplikasi mulai")
	dailylog.Debugf("versi=%s pid=%d", "1.0.0", 1234)
	dailylog.Error("contoh error:", errors.New("koneksi database gagal"))

	// 3) Aman dipakai dari banyak goroutine.
	var wg sync.WaitGroup
	for i := 1; i <= 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			dailylog.Infof("worker %d selesai", id)
		}(i)
	}
	wg.Wait()

	// 4) Ubah setelan saat runtime.
	dailylog.SetLevelEnabled(dailylog.LevelDebug, false)
	dailylog.Debug("pesan ini tidak muncul")
	dailylog.SetConsoleEnabled(false)
	dailylog.Info("pesan ini hanya masuk ke file")
	dailylog.SetConsoleEnabled(true)

	// 5) Atau pakai instance sendiri (tanpa logger global).
	custom, err := dailylog.New(
		dailylog.WithDir("logs/worker"),
		dailylog.WithConsoleEnabled(false),
		dailylog.WithCallerInfo(true),
	)
	if err != nil {
		panic(err)
	}
	defer custom.Close()
	custom.Info("log dari instance terpisah")

	dailylog.Info("aplikasi selesai")
}
