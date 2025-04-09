package log

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"runtime"
)

/*
包级别的 SetOutput 函数
允许开发者在应用程序的任何地方设置日志输出的目标，
而无需每次都创建一个新的 Logger 实例。
通过提供包级别的 SetOutput 函数，开发者可以更方便地配置日志输出，
而不需要每次都通过 Logger 实例来调用 SetOutput 方法。
这对于那些只需要一个全局日志记录器的应用程序来说非常有用。
*/

/*
这篇代码的主要目的是通过封装 logrus.Entry 的方法，
提供一个更灵活和一致的日志记录接口，
并通过包级别的日志记录功能，
使得整个应用程序可以使用统一的日志配置和记录方式。
这种方式不仅简化了日志记录的使用，
还增强了代码的可维护性和一致性。
*/

/*
主要功能
封装 logrus.Entry 的方法：
代码通过定义 Logger 结构体，并实现 ILogger 接口，
将 logrus.Entry 的方法进行了封装。
封装的方法参数使用小写字母开头，符合 Go 语言的命名规范。
这些封装的方法包括不同级别的日志记录方法
（如 Trace, Debug, Info, Warning, Error, Fatal, Panic）及其格式化版本
（如 TraceF, DebugF, InfoF, WarningF, ErrorF, FatalF, PanicF）。
包级别的日志记录功能：
代码定义了一个包级别的 Logger 实例 log，并在 init 函数中初始化它。
提供了一系列包级别的函数
（如 SetOutput, SetPrintCaller, SetCaller, Trace, Debug, Info, Warning, Error, Fatal, Panic,
TraceF, DebugF, InfoF, WarningF, ErrorF, FatalF, PanicF, WithFields），
这些函数可以直接使用包级别的 Logger 实例进行日志记录。
*/

// ILogger 定义了日志记录器的接口，提供了日志记录的各种方法。
type ILogger interface {
	SetLevel(lvl string)
	SetOutput(writer io.Writer)
	SetPrintCaller(bool)
	SetCaller(Caller func() (file string, line int, funcName string, err error))
	Trace(args ...any)
	Debug(args ...any)
	Info(args ...any)
	Warning(args ...any)
	Error(args ...any)
	Fatal(args ...any)
	Panic(args ...any)
	TraceF(format string, args ...any)
	DebugF(format string, args ...any)
	InfoF(format string, args ...any)
	WarningF(format string, args ...any)
	ErrorF(format string, args ...any)
	FatalF(format string, args ...any)
	PanicF(format string, args ...any)
	WithFields(fields map[string]any) ILogger
}

// Logger 是日志记录器的具体实现，实现了ILogger接口。
type Logger struct {
	entry       *logrus.Entry
	level       string
	printCaller bool
	caller      func() (file string, line int, funcName string, err error)
}

// SetLevel 设置日志级别。
func (l *Logger) SetLevel(lvl string) {
	if lvl == "" {
		return
	}
	level, err := logrus.ParseLevel(lvl)
	if err != nil {
		l.level = lvl
		l.entry.Logger.Level = level
	}
}

// SetOutput 设置日志输出目标。
func (l *Logger) SetOutput(writer io.Writer) {
	l.entry.Logger.SetOutput(writer)
}

// SetPrintCaller 设置是否打印调用者信息。
func (l *Logger) SetPrintCaller(printCaller bool) {
	l.printCaller = printCaller
}

// SetCaller 设置获取调用者信息的函数。
func (l *Logger) SetCaller(caller func() (file string, line int, funcName string, err error)) {
	l.caller = caller
}

// getCallerInfo 获取调用者信息，用于在日志中包含这些信息。
func (l *Logger) getCallerInfo(level logrus.Level) map[string]any {
	mp := make(map[string]any)
	if l.printCaller == true || level != logrus.InfoLevel {
		file, line, funcName, err := l.caller()
		if err == nil {
			mp["file"] = fmt.Sprintf("%s:%d", file, line)
			mp["func"] = funcName
		}
	}
	return mp
}

// log 是Trace、Debug、Info等方法的内部实现，用于实际记录日志。
func (l *Logger) log(level logrus.Level, args ...any) {
	l.entry.WithFields(l.getCallerInfo(level)).Log(level, args...)
}

// logf 是TraceF、DebugF、InfoF等方法的内部实现，用于实际格式化并记录日志。
func (l *Logger) logf(level logrus.Level, format string, args ...any) {
	l.entry.WithFields(l.getCallerInfo(level)).Logf(level, format, args...)
}

// 以下方法提供了不同级别的日志记录功能。
func (l *Logger) Trace(args ...any) {
	l.log(logrus.TraceLevel, args...)
}

func (l *Logger) Debug(args ...interface{}) {
	l.log(logrus.DebugLevel, args...)
}

func (l *Logger) Info(args ...interface{}) {
	l.log(logrus.InfoLevel, args...)
}

func (l *Logger) Warning(args ...interface{}) {
	l.log(logrus.WarnLevel, args...)
}

func (l *Logger) Error(args ...interface{}) {
	l.log(logrus.ErrorLevel, args...)
}

func (l *Logger) Fatal(args ...interface{}) {
	l.log(logrus.FatalLevel, args...)
}

func (l *Logger) Panic(args ...interface{}) {
	l.log(logrus.PanicLevel, args...)
}

// 以下方法提供了不同级别的格式化日志记录功能。
func (l *Logger) TraceF(format string, args ...interface{}) {
	l.logf(logrus.TraceLevel, format, args...)
}

func (l *Logger) DebugF(format string, args ...interface{}) {
	l.logf(logrus.DebugLevel, format, args...)
}

func (l *Logger) InfoF(format string, args ...interface{}) {
	l.logf(logrus.InfoLevel, format, args...)
}

func (l *Logger) WarningF(format string, args ...interface{}) {
	l.logf(logrus.WarnLevel, format, args...)
}

func (l *Logger) ErrorF(format string, args ...interface{}) {
	l.logf(logrus.ErrorLevel, format, args...)
}

func (l *Logger) FatalF(format string, args ...interface{}) {
	l.logf(logrus.FatalLevel, format, args...)
}

func (l *Logger) PanicF(format string, args ...interface{}) {
	l.logf(logrus.PanicLevel, format, args...)
}

// WithFields 创建一个新的Logger实例，带有额外的字段。
func (l *Logger) WithFields(fields map[string]any) ILogger {
	entry := l.entry.WithFields(fields)
	return &Logger{entry: entry, level: l.level, printCaller: l.printCaller, caller: l.caller}
}

// log 是包级别的Logger实例，提供全局日志记录功能。
var log *Logger

// NewLogger 创建并返回一个新的Logger实例。
func NewLogger() ILogger {
	return newLogger()
}

// newLogger 是NewLogger的内部实现。
func newLogger() *Logger {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.AddHook(&errorHook{})
	logger := &Logger{
		entry:  logrus.NewEntry(log),
		caller: defaultCaller,
	}
	return logger
}

// init 初始化包级别的Logger实例。
func init() {
	log = newLogger()
}

// 以下方法提供了包级别的日志配置功能。
func SetLevel(lvl string) {
	if lvl == "" {
		return
	}
	level, err := logrus.ParseLevel(lvl)
	if err == nil {
		log.level = lvl
		log.entry.Logger.Level = level
	}
}
func SetOutput(writer io.Writer) {
	log.entry.Logger.SetOutput(writer)
}

func SetPrintCaller(printCaller bool) {
	log.printCaller = printCaller
}

func SetCaller(caller func() (file string, line int, funcName string, err error)) {
	log.caller = caller
}

// defaultCaller 是默认的获取调用者信息的函数实现。
func defaultCaller() (file string, line int, funcName string, err error) {
	pc, f, l, ok := runtime.Caller(4)
	if !ok {
		err = errors.New("caller failure")
		return
	}
	funcName = runtime.FuncForPC(pc).Name()
	file, line = f, l
	return
}

// 以下方法提供了包级别的不同级别的日志记录功能。
func Trace(args ...interface{}) {
	log.log(logrus.TraceLevel, args...)
}

func Debug(args ...interface{}) {
	log.log(logrus.DebugLevel, args...)
}

func Info(args ...interface{}) {
	log.log(logrus.InfoLevel, args...)
}

func Warning(args ...interface{}) {
	log.log(logrus.WarnLevel, args...)
}

func Error(args ...interface{}) {
	log.log(logrus.ErrorLevel, args...)
}

func Fatal(args ...interface{}) {
	log.log(logrus.FatalLevel, args...)
}

func Panic(args ...interface{}) {
	log.log(logrus.PanicLevel, args...)
}

// 以下方法提供了包级别的不同级别的格式化日志记录功能。
func TraceF(format string, args ...interface{}) {
	log.logf(logrus.TraceLevel, format, args...)
}

func DebugF(format string, args ...interface{}) {
	log.logf(logrus.DebugLevel, format, args...)
}

func InfoF(format string, args ...interface{}) {
	log.logf(logrus.InfoLevel, format, args...)
}

func WarningF(format string, args ...interface{}) {
	log.logf(logrus.WarnLevel, format, args...)
}

func ErrorF(format string, args ...interface{}) {
	log.logf(logrus.ErrorLevel, format, args...)
}

func FatalF(format string, args ...interface{}) {
	log.logf(logrus.FatalLevel, format, args...)
}

func PanicF(format string, args ...interface{}) {
	log.logf(logrus.PanicLevel, format, args...)
}

// WithFields 创建一个新的Logger实例，带有额外的字段。
func WithFields(fields map[string]interface{}) *Logger {
	entry := log.entry.WithFields(fields)
	return &Logger{entry: entry, level: log.level, printCaller: log.printCaller, caller: log.caller}
}
