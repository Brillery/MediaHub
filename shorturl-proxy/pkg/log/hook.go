package log

import (
	"github.com/sirupsen/logrus"
	nativeLog "log"
)

// errorHook 是一个实现了logrus.Hook接口的结构体，用于在日志记录时执行自定义逻辑。
type errorHook struct{}

// Levels 返回该Hook关注的日志级别。
// 这里我们关注的是Panic、Fatal和Error级别的日志。
func (*errorHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
	}
}

// Fire 在日志记录时被调用，执行自定义的日志处理逻辑。
// 该函数将日志消息和数据通过标准库的log包打印出来。
// 参数entry: 包含日志消息和数据的logrus.Entry对象。
// 返回值: 错误对象，如果执行自定义逻辑时发生错误。
func (*errorHook) Fire(entry *logrus.Entry) error {
	nativeLog.Println(entry.Message, entry.Data)
	return nil
}
