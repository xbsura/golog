package golog

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// RFC5424
const (
	LEVEL_EMERGENCY = iota
	LEVEL_ALERT
	LEVEL_CRITICAL // used for always
	LEVEL_ERROR
	LEVEL_WARNING
	LEVEL_NOTICE // 5
	LEVEL_INFO
	LEVEL_DEBUG
	LEVEL_VERBOSE // 8
)

var (
	levelStrings = []string{
		"[EMERGENCY]",
		"[ALERT]",
		"[CRITICAL]",
		"[ERROR]",
		"[WARNING]",
		"[NOTICE]",
		"[INFO]",
		"[DEBUG]",
		"[VERB]",
	}
)

// A Logger represents an active logging object that generates lines of
// output to an io.Writer.  Each logging operation makes a single call to
// the Writer's Write method.  A Logger can be used simultaneously from
// multiple goroutines; it guarantees to serialize access to the Writer.
type Logger struct {
	level        int32
	mu           sync.Mutex // ensures atomic writes; protects the following fields
	out          *os.File   // destination for output
	path         string     // log file path
	buf          []byte     // for accumulating text to write
	microseconds bool
	shortfile    bool
}

/*
 * global static var
 */
var _log = &Logger{
	out:          os.Stderr,
	level:        LEVEL_NOTICE,
	microseconds: true,
	shortfile:    true,
}

var saveTime time.Duration = 0 * time.Second

func SetLevel(level int32) {
	Critical("set log level to %v", level)
	atomic.StoreInt32(&_log.level, level)
}

func GetLevel() int32 {
	v := atomic.LoadInt32(&_log.level)
	return v
}

func SetFile(path string) {
	//Critical("set log file to %v", path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		Error("error on SetLogFile: err: %s", err)
	}

	_log.out = f
	_log.path = path
}

func ReOpen(path string) {
	if _log.path == "" {
		return
	}
	_log.mu.Lock()
	defer _log.mu.Unlock()

	_log.out.Close()
	SetFile(_log.path)
}

func timestr(period time.Duration) string {
	t := time.Now().Add(time.Second * -10)

	if period == time.Minute {
		return fmt.Sprintf("%04d%02d%02d%02d%02d",
			t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute())
	}
	if period == time.Hour {
		return fmt.Sprintf("%04d%02d%02d%02d",
			t.Year(), t.Month(), t.Day(), t.Hour())
	}
	if period == time.Hour*24 {
		return fmt.Sprintf("%04d%02d%02d",
			t.Year(), t.Month(), t.Day())
	}

	return fmt.Sprintf("%04d%02d%02d%02d%02d%02d",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}

/*
 * enable rotate whit peirod
 * peirod can be: time.Minute, time.Hour, 24 * time.Hour
 */
func EnableRotate(period time.Duration) {
	if period != time.Minute && period != time.Hour && period != time.Hour*24 {
		Error("bad rotate peirod: %s", period)
		return
	}

	ch := make(chan bool)

	go func() {
		for {
			now := time.Now()
			nextRotateTime := now.Truncate(period).Add(period).Add(time.Second)
			timer := time.NewTimer(nextRotateTime.Sub(now))
			<-timer.C
			ch <- true
		}
	}()

	go func() {
		for {
			<-ch
			filename := fmt.Sprintf("%s.%s", _log.path, timestr(period))
			os.Rename(_log.path, filename)
			ReOpen(_log.path)
			go deleteExpiredLog(period)
		}
	}()
}

func SetLogSaveTime(period time.Duration) {
	saveTime = period
}

func deleteExpiredLog(period time.Duration) {
	dirName := filepath.Dir(_log.path)
	logName := filepath.Base(_log.path)
	fileInfos, err := ioutil.ReadDir(dirName)
	if err != nil {
		Warn("read dir %s fail, err is %v", dirName, err)
	}

	for _, fileInfo := range fileInfos {
		fileName := fileInfo.Name()
		mtime := fileInfo.ModTime()
		if saveTime != 0*time.Second &&
			strings.Index(fmt.Sprintf("%s.", fileName), logName) == 0 &&
			time.Now().Sub(mtime) >= saveTime {
			os.Remove(fmt.Sprintf("%s/%s", dirName, fileName))
		}
	}
}

func Critical(format string, v ...interface{}) {
	_log.output(LEVEL_CRITICAL, format, v...)
}

func Error(format string, v ...interface{}) {
	_log.output(LEVEL_ERROR, format, v...)
}

func Warn(format string, v ...interface{}) {
	_log.output(LEVEL_WARNING, format, v...)
}

func Notice(format string, v ...interface{}) {
	_log.output(LEVEL_NOTICE, format, v...)
}

func Info(format string, v ...interface{}) {
	_log.output(LEVEL_INFO, format, v...)
}

func Debug(format string, v ...interface{}) {
	_log.output(LEVEL_DEBUG, format, v...)
}

func Verbose(format string, v ...interface{}) {
	_log.output(LEVEL_VERBOSE, format, v...)
}

func Stacktrace(level int32, format string, v ...interface{}) {
	if level > GetLevel() {
		return
	}
	_log.output(level, format+" --- stack: \n%s", v, debug.Stack())
}

/*
 * variadic is slow because we create temp slices
 * so we add some help functions
 */
func Debug1(format string, a interface{}) {
	if LEVEL_DEBUG > GetLevel() {
		return
	}

	_log.output(LEVEL_DEBUG, format, a)
}

func Debug2(format string, a interface{}, b interface{}) {
	if LEVEL_DEBUG > GetLevel() {
		return
	}

	_log.output(LEVEL_DEBUG, format, a, b)
}

func Debug3(format string, a interface{}, b interface{}, c interface{}) {
	if LEVEL_DEBUG > GetLevel() {
		return
	}

	_log.output(LEVEL_DEBUG, format, a, b, c)
}

func Debug4(format string, a interface{}, b interface{}, c interface{}, d interface{}) {
	if LEVEL_DEBUG > GetLevel() {
		return
	}

	_log.output(LEVEL_DEBUG, format, a, b, c, d)
}

func Info1(format string, a interface{}) {
	if LEVEL_INFO > GetLevel() {
		return
	}

	_log.output(LEVEL_INFO, format, a)
}

func Info2(format string, a interface{}, b interface{}) {
	if LEVEL_INFO > GetLevel() {
		return
	}

	_log.output(LEVEL_INFO, format, a, b)
}

func Info3(format string, a interface{}, b interface{}, c interface{}) {
	if LEVEL_INFO > GetLevel() {
		return
	}

	_log.output(LEVEL_INFO, format, a, b, c)
}

func Info4(format string, a interface{}, b interface{}, c interface{}, d interface{}) {
	if LEVEL_INFO > GetLevel() {
		return
	}

	_log.output(LEVEL_INFO, format, a, b, c, d)
}

// Cheap integer to fixed-width decimal ASCII.
// Give a negative width to avoid zero-padding.
// Knows the buffer has capacity.
func itoa(buf *[]byte, i int, wid int) {
	var u uint = uint(i)
	if u == 0 && wid <= 1 {
		*buf = append(*buf, '0')
		return
	}

	// Assemble decimal in reverse order.
	var b [32]byte
	bp := len(b)
	for ; u > 0 || wid > 0; u /= 10 {
		bp--
		wid--
		b[bp] = byte(u%10) + '0'
	}
	*buf = append(*buf, b[bp:]...)
}

func (l *Logger) formatHeader(buf *[]byte, t time.Time,
	level int32, file string, line int) {

	//2015-05-14
	year, month, day := t.Date()
	itoa(buf, year, 4)
	*buf = append(*buf, '-')
	itoa(buf, int(month), 2)
	*buf = append(*buf, '-')
	itoa(buf, day, 2)
	*buf = append(*buf, ' ')

	//09:56:00.023132
	hour, min, sec := t.Clock()
	itoa(buf, hour, 2)
	*buf = append(*buf, ':')
	itoa(buf, min, 2)
	*buf = append(*buf, ':')
	itoa(buf, sec, 2)
	if l.microseconds {
		*buf = append(*buf, '.')
		itoa(buf, t.Nanosecond()/1e3, 6)
	}
	*buf = append(*buf, ' ')

	// [DEBUG] level
	*buf = append(*buf, levelStrings[level]...)
	*buf = append(*buf, ' ')

	// xxx.go (filename)
	short := file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	file = short

	*buf = append(*buf, file...)
	*buf = append(*buf, ':')
	itoa(buf, line, -1)
	*buf = append(*buf, ": "...)
}

func (l *Logger) output(level int32, format string, v ...interface{}) error {
	if level > GetLevel() {
		return nil
	}

	s := fmt.Sprintf(format, v...)

	now := time.Now() // get this early.
	var file string
	var line int
	l.mu.Lock()
	defer l.mu.Unlock()

	// release lock while getting caller info - it's expensive.
	l.mu.Unlock()
	var ok bool
	_, file, line, ok = runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
	}
	l.mu.Lock()

	l.buf = l.buf[:0]
	l.formatHeader(&l.buf, now, level, file, line)
	l.buf = append(l.buf, s...)
	if len(s) > 0 && s[len(s)-1] != '\n' {
		l.buf = append(l.buf, '\n')
	}
	_, err := l.out.Write(l.buf)
	return err
}
