// Package dailylog adalah logger sederhana dan aman untuk banyak goroutine
// dengan fitur:
//
//   - file log harian (yyyy-mm-dd.log) di satu folder khusus
//   - nama file dan nomor baris pemanggil (mis. main.go:42)
//   - level DEBUG, INFO, ERROR yang bisa di-enable/disable
//   - output ke file dan/atau console
//   - penghapusan otomatis file log yang lebih tua dari N hari
//
// Contoh singkat:
//
//	if err := dailylog.Init(dailylog.WithDir("logs")); err != nil {
//		panic(err)
//	}
//	defer dailylog.Close()
//
//	dailylog.Info("server jalan di port", 8080)
//	dailylog.Errorf("gagal koneksi: %v", err)
package dailylog

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Level adalah tingkat/jenis log.
type Level int

// Daftar level yang tersedia.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelError

	numLevels = 3
)

var levelLabels = [numLevels]string{"DEBUG", "INFO ", "ERROR"}

var levelColors = [numLevels]string{
	"\x1b[36m", // DEBUG: cyan
	"\x1b[32m", // INFO : hijau
	"\x1b[31m", // ERROR: merah
}

const colorReset = "\x1b[0m"

// String mengembalikan nama level (tanpa padding).
func (l Level) String() string {
	if l < 0 || int(l) >= numLevels {
		return "UNKNOWN"
	}
	return strings.TrimSpace(levelLabels[l])
}

const (
	fileDateLayout = "2006-01-02"
	fileNameLayout = fileDateLayout + ".log"
	fullTimeLayout = "2006-01-02 15:04:05.000"
	timeLayout     = "15:04:05"
)

// logFileRe hanya cocok dengan nama file log milik package ini, sehingga file
// lain di folder yang sama tidak pernah ikut terhapus.
var logFileRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\.log$`)

// Logger menulis log ke file harian dan/atau console.
// Semua method aman dipanggil dari banyak goroutine sekaligus.
type Logger struct {
	// mu melindungi file, fileDate, closed, dan urutan penulisan output.
	mu       sync.Mutex
	file     *os.File
	fileDate string
	closed   bool

	dir        string
	color      bool
	callerSkip int
	stdout     io.Writer
	stderr     io.Writer
	now        func() time.Time // bisa diganti saat testing

	fileEnabled        atomic.Bool
	consoleEnabled     atomic.Bool
	FullTimeLayout     atomic.Bool
	FullFilenameLayout atomic.Bool
	CallerInfo         atomic.Bool
	levelEnabled       [numLevels]atomic.Bool
	maxAgeDays         atomic.Int64

	stop      chan struct{}
	wg        sync.WaitGroup
	closeOnce sync.Once
}

// New membuat Logger baru.
//
// Saat dibuat, folder log disiapkan dan file log yang lebih tua dari
// MaxAgeDays langsung dihapus. Setelah itu, jika CleanupInterval > 0,
// sebuah goroutine akan mengulang pembersihan secara berkala sampai Close
// dipanggil.
func New(opts ...Option) (*Logger, error) {
	o := defaultOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&o)
		}
	}
	if strings.TrimSpace(o.dir) == "" {
		return nil, errors.New("dailylog: dir tidak boleh kosong")
	}
	if o.stdout == nil {
		o.stdout = os.Stdout
	}
	if o.stderr == nil {
		o.stderr = os.Stderr
	}

	l := &Logger{
		dir:        o.dir,
		color:      o.color,
		callerSkip: o.callerSkip,
		stdout:     o.stdout,
		stderr:     o.stderr,
		now:        time.Now,
		stop:       make(chan struct{}),
	}
	l.fileEnabled.Store(o.fileEnabled)
	l.consoleEnabled.Store(o.consoleEnabled)
	l.FullTimeLayout.Store(o.FullTimeLayout)
	l.FullFilenameLayout.Store(o.FullFilenameLayout)
	l.CallerInfo.Store(o.CallerInfo)

	for i := 0; i < numLevels; i++ {
		l.levelEnabled[i].Store(o.levels[i])
	}
	l.maxAgeDays.Store(int64(o.maxAgeDays))

	if o.fileEnabled {
		if err := os.MkdirAll(l.dir, 0o755); err != nil {
			return nil, fmt.Errorf("dailylog: gagal membuat folder log %q: %w", l.dir, err)
		}
	}

	// Pembersihan pertama: sinkron, saat logger mulai.
	if l.maxAgeDays.Load() > 0 && o.cleanupInterval > 0 {
		if _, err := l.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "dailylog: cleanup awal gagal: %v\n", err)
		}
	}

	// Pembersihan berikutnya: goroutine dengan interval.
	if o.cleanupInterval > 0 && l.maxAgeDays.Load() > 0 {
		l.wg.Add(1)
		go l.cleanupLoop(o.cleanupInterval)
	}
	return l, nil
}

func (l *Logger) cleanupLoop(interval time.Duration) {
	defer l.wg.Done()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if _, err := l.Cleanup(); err != nil {
				fmt.Fprintf(os.Stderr, "dailylog: cleanup gagal: %v\n", err)
			}
		case <-l.stop:
			return
		}
	}
}

// Cleanup menghapus file log (yyyy-mm-dd.log) yang umurnya lebih dari
// MaxAgeDays hari dan mengembalikan jumlah file yang terhapus.
// Hanya file dengan pola nama yyyy-mm-dd.log yang disentuh.
// Aman dipanggil kapan saja, termasuk saat logger sedang dipakai.
func (l *Logger) Cleanup() (int, error) {
	days := int(l.maxAgeDays.Load())
	if days <= 0 {
		return 0, nil
	}
	entries, err := os.ReadDir(l.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	now := l.now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	cutoff := today.AddDate(0, 0, -days)

	var (
		removed int
		errs    []error
	)
	for _, e := range entries {
		if e.IsDir() || !logFileRe.MatchString(e.Name()) {
			continue
		}
		fileDay, err := time.ParseInLocation(fileNameLayout, e.Name(), now.Location())
		if err != nil {
			continue
		}
		if fileDay.Before(cutoff) {
			if err := os.Remove(filepath.Join(l.dir, e.Name())); err != nil {
				if !os.IsNotExist(err) {
					errs = append(errs, err)
				}
				continue
			}
			removed++
		}
	}
	return removed, errors.Join(errs...)
}

// Close menghentikan goroutine cleanup dan menutup file log.
// Aman dipanggil lebih dari sekali. Setelah Close, log tidak lagi ditulis ke
// file (console tetap berfungsi bila diaktifkan).
func (l *Logger) Close() error {
	var err error
	l.closeOnce.Do(func() {
		close(l.stop)
		l.wg.Wait()

		l.mu.Lock()
		defer l.mu.Unlock()
		l.closed = true
		if l.file != nil {
			err = l.file.Close()
			l.file = nil
		}
	})
	return err
}

// ---- Pengaturan saat runtime (semuanya thread-safe) ------------------------

// SetFileEnabled mengaktifkan/menonaktifkan penulisan ke file.
func (l *Logger) SetFileEnabled(enabled bool) { l.fileEnabled.Store(enabled) }

// SetConsoleEnabled mengaktifkan/menonaktifkan print ke console.
func (l *Logger) SetConsoleEnabled(enabled bool) { l.consoleEnabled.Store(enabled) }

// SetLevelEnabled mengaktifkan/menonaktifkan satu level log.
func (l *Logger) SetLevelEnabled(level Level, enabled bool) {
	if level >= 0 && int(level) < numLevels {
		l.levelEnabled[level].Store(enabled)
	}
}

// LevelEnabled melaporkan apakah suatu level sedang aktif.
func (l *Logger) LevelEnabled(level Level) bool {
	if level < 0 || int(level) >= numLevels {
		return false
	}
	return l.levelEnabled[level].Load()
}

// SetMaxAgeDays mengubah umur maksimal file log. Nilai <= 0 menonaktifkan
// penghapusan otomatis. Berlaku pada pembersihan berikutnya.
func (l *Logger) SetMaxAgeDays(days int) { l.maxAgeDays.Store(int64(days)) }

// ---- API logging -----------------------------------------------------------

// Debug mencatat log level DEBUG. Argumen digabung seperti fmt.Sprintln.
func (l *Logger) Debug(args ...any) {
	if !l.LevelEnabled(LevelDebug) {
		return
	}
	l.output(LevelDebug, sprint(args))
}

// Debugf mencatat log level DEBUG dengan format fmt.
func (l *Logger) Debugf(format string, args ...any) {
	if !l.LevelEnabled(LevelDebug) {
		return
	}
	l.output(LevelDebug, fmt.Sprintf(format, args...))
}

// Info mencatat log level INFO. Argumen digabung seperti fmt.Sprintln.
func (l *Logger) Info(args ...any) {
	if !l.LevelEnabled(LevelInfo) {
		return
	}
	l.output(LevelInfo, sprint(args))
}

// Infof mencatat log level INFO dengan format fmt.
func (l *Logger) Infof(format string, args ...any) {
	if !l.LevelEnabled(LevelInfo) {
		return
	}
	l.output(LevelInfo, fmt.Sprintf(format, args...))
}

// Error mencatat log level ERROR. Argumen digabung seperti fmt.Sprintln.
func (l *Logger) Error(args ...any) {
	if !l.LevelEnabled(LevelError) {
		return
	}
	l.output(LevelError, sprint(args))
}

// Errorf mencatat log level ERROR dengan format fmt.
func (l *Logger) Errorf(format string, args ...any) {
	if !l.LevelEnabled(LevelError) {
		return
	}
	l.output(LevelError, fmt.Sprintf(format, args...))
}

func sprint(args []any) string {
	return strings.TrimSuffix(fmt.Sprintln(args...), "\n")
}

// output adalah satu-satunya tempat log ditulis. Dipanggil langsung oleh
// method publik (Debug, Info, ...) dan fungsi package-level, sehingga
// runtime.Caller(2) selalu menunjuk ke kode pemanggil milik pengguna.
func (l *Logger) output(level Level, msg string) {
	if !l.levelEnabled[level].Load() {
		return
	}
	toFile := l.fileEnabled.Load()
	toConsole := l.consoleEnabled.Load()
	if !toFile && !toConsole {
		return
	}

	// 0 = output, 1 = Info/Debug/Error(f), 2 = pemanggil.
	loc := ""
	if l.CallerInfo.Load() {
		loc = "???:0"
		if _, file, line, ok := runtime.Caller(2 + l.callerSkip); ok {
			if l.FullFilenameLayout.Load() {
				loc = fmt.Sprintf("%s:%d", file, line)
			} else {
				loc = fmt.Sprintf("%s:%d", filepath.Base(file), line)
			}
		}
	}

	now := l.now()
	var ts string
	if l.FullTimeLayout.Load() {
		ts = now.Format(fullTimeLayout)
	} else {
		ts = now.Format(timeLayout)
	}
	label := levelLabels[level]

	var text string
	if loc != "" {
		text = ts + " [" + label + "] " + loc + ": " + msg + "\n"
	} else {
		text = ts + " [" + label + "] " + msg + "\n"
	}

	//text := ts + " [" + label + "] " + loc + ": " + msg + "\n"

	l.mu.Lock()
	defer l.mu.Unlock()

	if toFile {
		l.writeFileLocked(now, text)
	}
	if toConsole {
		out, console := l.stdout, text
		if level == LevelError {
			out = l.stderr
		}
		if l.color {
			console = ts + " " + levelColors[level] + "[" + label + "]" + colorReset + " " + loc + ": " + msg + "\n"
		}
		_, _ = io.WriteString(out, console)
	}
}

// writeFileLocked menulis ke file harian; berganti file otomatis saat tanggal
// berubah. Harus dipanggil dengan l.mu terkunci.
func (l *Logger) writeFileLocked(now time.Time, text string) {
	if l.closed {
		return
	}
	date := now.Format(fileDateLayout)
	if l.file == nil || l.fileDate != date {
		if l.file != nil {
			_ = l.file.Close()
			l.file = nil
		}
		if err := os.MkdirAll(l.dir, 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "dailylog: gagal membuat folder %q: %v\n", l.dir, err)
			return
		}
		path := filepath.Join(l.dir, date+".log")
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dailylog: gagal membuka %q: %v\n", path, err)
			return
		}
		l.file = f
		l.fileDate = date
	}
	if _, err := l.file.WriteString(text); err != nil {
		fmt.Fprintf(os.Stderr, "dailylog: gagal menulis log: %v\n", err)
	}
}
