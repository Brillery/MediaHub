package log

import (
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"sync"
)

/*
实际工作流程
初始写入：
当你第一次调用 GetRotateWriter(logPath) 时，如果没有找到与 logPath 关联的写入器，则会创建一个新的 lumberjack.Logger 实例，并将其与路径关联。
日志写入：
每次写入日志时，日志信息会被追加到当前的日志文件中。
触发轮转：
当日志文件大小超过 MaxSize 或者达到了其他轮转条件（如时间周期），lumberjack.Logger 会关闭当前文件并创建一个新的日志文件继续写入。
清理旧日志：
如果旧的日志文件数量超过了 MaxBackups 或者文件年龄超过了 MaxAge，这些旧文件会被自动删除。
*/

// fileRotateWriter 是一个结构体，用于管理和提供对日志文件的轮转写入功能。
// 它包含一个映射，用于存储日志路径和对应的写入器，以及一个读写锁，用于并发控制。
type fileRotateWriter struct {
	data map[string]io.Writer
	sync.RWMutex
}

// getWriter 方法用于获取与指定日志路径关联的写入器。
// 它使用读锁来最小化对并发访问的影响。
func (frw *fileRotateWriter) getWriter(logPath string) io.Writer {
	frw.RLock()
	defer frw.RUnlock()
	w, ok := frw.data[logPath]
	if !ok {
		return nil
	}
	return w
}

// setWriter 方法用于设置或更新指定日志路径的写入器。
// 它使用写锁来确保线程安全。
func (frw *fileRotateWriter) setWriter(logPath string, w io.Writer) io.Writer {
	frw.Lock()
	defer frw.Unlock()
	frw.data[logPath] = w
	return w
}

// _fileRotateWriter 是一个全局变量，表示fileRotateWriter类型的实例。
var _fileRotateWriter *fileRotateWriter

// init 函数用于初始化_fileRotateWriter实例。
func init() {
	_fileRotateWriter = &fileRotateWriter{
		data: map[string]io.Writer{},
	}
}

// GetRotateWriter 函数用于获取与指定路径关联的日志文件写入器。
// 如果该路径尚未关联任何写入器，则创建一个新的lumberjack.Logger实例并关联。
func GetRotateWriter(logPath string) io.Writer {
	if logPath == "" {
		panic("日志文件路径不能为空")
	}
	writer := _fileRotateWriter.getWriter(logPath)
	if writer != nil {
		return writer
	}
	writer = &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    1,    // 最大日志文件大小为1MB
		MaxBackups: 15,   // 保留最多15个备份文件
		MaxAge:     7,    // 最大保存天数为7天
		LocalTime:  true, // 使用本地时间
	}
	return _fileRotateWriter.setWriter(logPath, writer)
}
