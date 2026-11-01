package dailylog

import (
	"fmt"
	"os"
	"sync"
)

// Logger global, supaya bisa dipakai tanpa membawa-bawa instance:
//
//	dailylog.Info("halo")
var (
	defaultMu     sync.RWMutex
	defaultLogger *Logger
)

// Init membuat logger global dengan option yang diberikan. Jika sudah ada
// logger global sebelumnya, logger lama ditutup. Panggil di awal program,
// dan `defer dailylog.Close()`.
/*func Init(opts ...Option) error {
	l, err := New(opts...)
	if err != nil {
		return err
	}
	defaultMu.Lock()
	old := defaultLogger
	defaultLogger = l
	defaultMu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}
*/

// init dengan DailyLogConfig default. Jika sudah ada logger global sebelumnya,
// logger lama ditutup. Panggil di awal program, dan `defer dailylog.Close()`.
func Init(cfg DailyLogConfig) error {
	l, err := New(
		WithDir(cfg.Dir),
		WithFileEnabled(cfg.FileEnabled),
		WithConsoleEnabled(cfg.ConsoleEnabled),
		WithMaxAgeDays(cfg.MaxAgeDays),
		WithCleanupInterval(cfg.CleanupInterval),
		FullTimeLayout(cfg.FullTimeLayout),
		FullFilenameLayout(cfg.FullFilenameLayout),
		WithCallerInfo(cfg.CallerInfo),
	)
	if err != nil {
		return err
	}
	defaultMu.Lock()
	old := defaultLogger
	defaultLogger = l
	defaultMu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

// Default mengembalikan logger global. Jika Init belum dipanggil, logger
// dibuat otomatis dengan konfigurasi default (folder "logs", file+console).
func Default() *Logger {
	defaultMu.RLock()
	l := defaultLogger
	defaultMu.RUnlock()
	if l != nil {
		return l
	}

	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultLogger == nil {
		nl, err := New()
		if err != nil {
			fmt.Fprintf(os.Stderr, "dailylog: %v (log ke file dimatikan)\n", err)
			nl, _ = New(WithFileEnabled(false))
		}
		defaultLogger = nl
	}
	return defaultLogger
}

// Close menutup logger global (goroutine cleanup dan file).
func Close() error {
	defaultMu.RLock()
	l := defaultLogger
	defaultMu.RUnlock()
	if l == nil {
		return nil
	}
	return l.Close()
}

// Catatan: fungsi-fungsi di bawah memanggil output() secara langsung (bukan
// lewat method Logger) agar nama file:baris yang tercatat adalah milik
// pemanggil, bukan milik package ini.

// Debug mencatat log DEBUG ke logger global.
func Debug(args ...any) {
	if l := Default(); l.LevelEnabled(LevelDebug) {
		l.output(LevelDebug, sprint(args))
	}
}

// Debugf mencatat log DEBUG berformat ke logger global.
func Debugf(format string, args ...any) {
	if l := Default(); l.LevelEnabled(LevelDebug) {
		l.output(LevelDebug, fmt.Sprintf(format, args...))
	}
}

// Info mencatat log INFO ke logger global.
func Info(args ...any) {
	if l := Default(); l.LevelEnabled(LevelInfo) {
		l.output(LevelInfo, sprint(args))
	}
}

// Infof mencatat log INFO berformat ke logger global.
func Infof(format string, args ...any) {
	if l := Default(); l.LevelEnabled(LevelInfo) {
		l.output(LevelInfo, fmt.Sprintf(format, args...))
	}
}

// Error mencatat log ERROR ke logger global.
func Error(args ...any) {
	if l := Default(); l.LevelEnabled(LevelError) {
		l.output(LevelError, sprint(args))
	}
}

// Errorf mencatat log ERROR berformat ke logger global.
func Errorf(format string, args ...any) {
	if l := Default(); l.LevelEnabled(LevelError) {
		l.output(LevelError, fmt.Sprintf(format, args...))
	}
}

// SetFileEnabled mengaktifkan/menonaktifkan log ke file pada logger global.
func SetFileEnabled(enabled bool) { Default().SetFileEnabled(enabled) }

// SetConsoleEnabled mengaktifkan/menonaktifkan console pada logger global.
func SetConsoleEnabled(enabled bool) { Default().SetConsoleEnabled(enabled) }

// SetLevelEnabled mengaktifkan/menonaktifkan satu level pada logger global.
func SetLevelEnabled(level Level, enabled bool) { Default().SetLevelEnabled(level, enabled) }
