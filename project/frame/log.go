package frame

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

func GetNextHourIntervalS() int {
	return int(3600 - (TimeStamp % 3600))
}

func GenLogFileName() string {
	dateStr := time.Now().Format(time.DateTime)
	r := strings.NewReplacer(":", "", "-", "", " ", "")
	return r.Replace(dateStr)
}

type LogLevel int

const (
	LogLevelDebug = 1
	LogLevelInfo  = 2
	LogLevelWarn  = 3
	LogLevelError = 4
)

type ILogger interface {
	LogWrite(str string)
}

type ConsoleLogger struct {
}

func (logger *ConsoleLogger) LogWrite(str string) {
	fmt.Println(str)
}

type FileLogger struct {
	Path           string
	size           int
	file           *os.File
	fileName       string // 文件名
	extensionName  string // 拓展名
	dictionaryName string // 目录名
}

func (logger *FileLogger) LogWrite(str string) {
	if logger.file == nil {
		return
	}

	// file size max = 10mb
	if logger.size > 1024*1024*10 {
		_ = logger.file.Close()
		logger.file = nil
		newPath := logger.dictionaryName + "/" + logger.fileName + GenLogFileName() + logger.extensionName
		_ = os.Rename(logger.Path, newPath)
		file, err := os.OpenFile(logger.Path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
		if err == nil {
			logger.file = file
		}
		logger.size = 0
	}

	_, _ = logger.file.WriteString(str)
	_, _ = logger.file.WriteString("\n")
	logger.size += len(str) + 1
}

type Log struct {
	logger         [8]ILogger
	loggerCount    int
	buffSize       int
	writeChannel   chan string
	recoverChannel chan *FileLogger
	formatFunc     func(level LogLevel, fileName string, params ...interface{}) string
	stopFlag       int32
	logLevel       LogLevel
}

func (r *Log) IsStop() bool {
	if r.stopFlag == 0 && LogEnd() {
		r.Stop()
	}
	return r.stopFlag == 1
}

func (r *Log) Stop() {
	if atomic.CompareAndSwapInt32(&r.stopFlag, 0, 1) {
		close(r.writeChannel)
		close(r.recoverChannel)
	}
}

func (r *Log) write(levelStr string, level LogLevel, params ...interface{}) {
	defer func() { recover() }()
	if r.IsStop() {
		return
	}

	_, file, line, ok := runtime.Caller(3) // 1.write 2.log 3.fun
	if ok {
		i := strings.LastIndex(file, "/") + 1
		if r.formatFunc != nil {
			r.writeChannel <- r.formatFunc(level, string(([]byte(file))[i:]), line, params)
		} else {
			logStr := fmt.Sprintf("[%v][%v][%v:%v]:", levelStr, time.Now().Format(time.DateTime), string(([]byte(file))[i:]), line)
			r.writeChannel <- logStr + fmt.Sprintf(params[0].(string), params[1:]...)
		}
	}
}

func (r *Log) start() {
	goForLogger(func(stopCh chan struct{}) {
		defer func() {
			// 处理剩余日志
			for str := range r.writeChannel {
				for i := 0; i < r.loggerCount; i++ {
					r.logger[i].LogWrite(str)
				}
			}
			// 关闭文件句柄
			for i := 0; i < r.loggerCount; i++ {
				if fl, ok := r.logger[i].(*FileLogger); ok {
					if fl.file != nil {
						_ = fl.file.Close()
						fl.file = nil
					}
				}
			}
		}()

		for !r.IsStop() {
			select {
			case str, ok := <-r.writeChannel:
				if !ok {
					break
				}
				for i := 0; i < r.loggerCount; i++ {
					r.logger[i].LogWrite(str)
				}

			case fl, ok := <-r.recoverChannel:
				if !ok {
					break
				}
				_ = fl.file.Close()
				fl.file = nil

				// 轮换新文件名
				newPath := fl.dictionaryName + "/" + fl.fileName + GenLogFileName() + fl.extensionName
				_ = os.Rename(fl.Path, newPath)
				file, err := os.OpenFile(fl.Path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
				if err == nil {
					fl.file = file
				}
				fl.size = 0

				timeout := GetNextHourIntervalS()
				Gogo(func() {
					timer := time.NewTimer(time.Duration(timeout) * time.Second)
					select {
					case <-stopChForGo:
					case <-timer.C:
						timer.Stop()
						r.recoverChannel <- fl
					}
				})

			case <-stopCh:

			}
		}
	})
}

func (r *Log) initFileLogger(f *FileLogger) *FileLogger {
	f.Path, _ = filepath.Abs(f.Path)
	f.Path = strings.Replace(f.Path, "\\", "/", -1)
	f.extensionName = path.Ext(f.Path)
	f.dictionaryName = path.Dir(f.Path)
	f.fileName = filepath.Base(f.Path[:len(f.Path)-len(f.extensionName)])
	err := os.MkdirAll(f.dictionaryName, os.ModePerm)
	if err != nil {
		return nil
	}
	file, err := os.OpenFile(f.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil
	}
	f.file = file
	info, err := file.Stat()
	if err != nil {
		return nil
	}
	f.size = int(info.Size())

	timeout := GetNextHourIntervalS()
	Gogo(func() {
		timer := time.NewTimer(time.Duration(timeout) * time.Second)
		select {
		case <-stopChForGo:
		case <-timer.C:
			timer.Stop()
			r.recoverChannel <- f // 小时制回收
		}
	})

	return f
}

func (r *Log) SetLevel(level LogLevel) {
	r.logLevel = level
}

func (r *Log) SetFormatFunc(f func(LogLevel, string, ...interface{}) string) {
	r.formatFunc = f
}

func (r *Log) Debug(params ...interface{}) {
	if r.logLevel >= LogLevelDebug {
		r.write("D", r.logLevel, params...)
	}
}

func (r *Log) Info(params ...interface{}) {
	if r.logLevel >= LogLevelInfo {
		r.write("I", r.logLevel, params...)
	}
}

func (r *Log) Warn(params ...interface{}) {
	if r.logLevel >= LogLevelWarn {
		r.write("W", r.logLevel, params...)
	}
}

func (r *Log) Error(params ...interface{}) {
	if r.logLevel >= LogLevelError {
		r.write("D", r.logLevel, params...)
	}
}

func NewLog(buffSize int, loggers ...ILogger) *Log {
	logObject := &Log{
		loggerCount:    0,
		buffSize:       buffSize,
		writeChannel:   make(chan string, buffSize),
		recoverChannel: make(chan *FileLogger, 32),
	}
	for _, logger := range loggers {
		if logObject.loggerCount == 8 {
			break
		}
		if f, ok := logger.(*FileLogger); ok {
			logObject.initFileLogger(f)
		}
		logObject.logger[logObject.loggerCount] = logger
		logObject.loggerCount++
	}
	logObject.start()
	return logObject
}

func LogDebug(params ...interface{}) {
	defaultLogger.Debug(params...)
}

func LogInfo(params ...interface{}) {
	defaultLogger.Info(params...)
}

func LogWarn(params ...interface{}) {
	defaultLogger.Warn(params...)
}

func LogError(params ...interface{}) {
	defaultLogger.Error(params...)
}

func LogPanic(params ...interface{}) {
	buf := make([]byte, 1<<12)
	LogError(string(buf[:runtime.Stack(buf, false)]))
}
