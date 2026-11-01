package dailylog

import (
	"io"
	"time"
)

// Nilai default yang dipakai bila option tidak diberikan.
const (
	// DefaultDir adalah folder log default (relatif terhadap working directory).
	DefaultDir = "logs"
	// DefaultMaxAgeDays adalah umur maksimal file log (hari) sebelum dihapus.
	DefaultMaxAgeDays = 30
	// DefaultCleanupInterval adalah jeda antar pembersihan file log lama.
	DefaultCleanupInterval = 24 * time.Hour
)

type DailyLogConfig struct {
	Dir                string
	FileEnabled        bool
	ConsoleEnabled     bool
	Levels             [numLevels]bool
	MaxAgeDays         int
	CleanupInterval    time.Duration
	FullTimeLayout     bool
	FullFilenameLayout bool
	CallerInfo         bool
}

type options struct {
	dir                string
	fileEnabled        bool
	consoleEnabled     bool
	levels             [numLevels]bool
	maxAgeDays         int
	cleanupInterval    time.Duration
	color              bool
	callerSkip         int
	FullTimeLayout     bool
	FullFilenameLayout bool
	CallerInfo         bool
	stdout             io.Writer
	stderr             io.Writer
}

func defaultOptions() options {
	return options{
		dir:                DefaultDir,
		fileEnabled:        true,
		consoleEnabled:     true,
		levels:             [numLevels]bool{true, true, true},
		maxAgeDays:         DefaultMaxAgeDays,
		cleanupInterval:    DefaultCleanupInterval,
		FullTimeLayout:     false,
		FullFilenameLayout: false,
		CallerInfo:         true,
	}
}

// Option mengubah konfigurasi Logger. Dipakai pada New dan Init.
type Option func(*options)

// WithDir menentukan folder tempat semua file log disimpan.
// Folder dibuat otomatis jika belum ada. Default: "logs".
func WithDir(dir string) Option {
	return func(o *options) { o.dir = dir }
}

// WithFileEnabled mengaktifkan/menonaktifkan penulisan ke file. Default: true.
func WithFileEnabled(enabled bool) Option {
	return func(o *options) { o.fileEnabled = enabled }
}

// WithConsoleEnabled mengaktifkan/menonaktifkan print ke console. Default: true.
func WithConsoleEnabled(enabled bool) Option {
	return func(o *options) { o.consoleEnabled = enabled }
}

// WithDebug mengaktifkan/menonaktifkan level DEBUG. Default: true.
func WithDebug(enabled bool) Option {
	return func(o *options) { o.levels[LevelDebug] = enabled }
}

// WithInfo mengaktifkan/menonaktifkan level INFO. Default: true.
func WithInfo(enabled bool) Option {
	return func(o *options) { o.levels[LevelInfo] = enabled }
}

// WithError mengaktifkan/menonaktifkan level ERROR. Default: true.
func WithError(enabled bool) Option {
	return func(o *options) { o.levels[LevelError] = enabled }
}

// WithMaxAgeDays mengatur umur maksimal file log dalam hari. File yang lebih tua
// dari ini akan dihapus. Nilai <= 0 menonaktifkan penghapusan otomatis.
// Default: 30.
func WithMaxAgeDays(days int) Option {
	return func(o *options) { o.maxAgeDays = days }
}

// WithCleanupInterval mengatur seberapa sering goroutine membersihkan file log
// lama. Pembersihan pertama selalu dijalankan sekali saat logger dibuat.
// Nilai <= 0 berarti hanya dibersihkan saat start. Default: 24 jam.
func WithCleanupInterval(d time.Duration) Option {
	return func(o *options) { o.cleanupInterval = d }
}

// WithColor mengaktifkan warna ANSI pada tag level di console. Default: false.
func WithColor(enabled bool) Option {
	return func(o *options) { o.color = enabled }
}

// WithCallerSkip menambah jumlah stack frame yang dilewati saat mencari
// nama file:baris. Berguna jika Logger dibungkus oleh fungsi helper milik Anda
// sehingga baris yang tampil seharusnya pemanggil helper tersebut.
func WithCallerSkip(skip int) Option {
	return func(o *options) { o.callerSkip = skip }
}

// WithConsoleWriters mengganti tujuan output console. Level DEBUG dan INFO
// ditulis ke out, level ERROR ditulis ke errOut. Nilai nil memakai
// os.Stdout / os.Stderr. Berguna untuk testing atau mengarahkan ke writer lain.
func WithConsoleWriters(out, errOut io.Writer) Option {
	return func(o *options) {
		o.stdout = out
		o.stderr = errOut
	}
}

func FullTimeLayout(enabled bool) Option {
	return func(o *options) {
		o.FullTimeLayout = enabled
	}
}

func FullFilenameLayout(enabled bool) Option {
	return func(o *options) {
		o.FullFilenameLayout = enabled
	}
}

func WithCallerInfo(enabled bool) Option {
	return func(o *options) {
		o.CallerInfo = enabled
	}
}
