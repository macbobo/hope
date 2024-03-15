package utils

import (
	"github.com/panjf2000/gnet/pkg/logging"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type loger_t struct {
	SugarLogger *zap.SugaredLogger
}

var Logger *loger_t

func Newloger(level logging.Level, logpath string, rotate bool, rps ...int) *loger_t {

	l := new(loger_t)
	var core zapcore.Core
	encoder := logerEncoder()
	if rotate {
		writeSyncer := logerWriter(logpath, rps)
		core = zapcore.NewCore(encoder, writeSyncer, level)
	} else {
		r := []int{1000, 0, 360, 0}
		writeSyncer := logerWriter(logpath, r)
		core = zapcore.NewCore(encoder, writeSyncer, level)
	}

	// zap.AddCaller()  添加将调用函数信息记录到日志中的功能。
	logger := zap.New(core, zap.AddCaller())
	l.SugarLogger = logger.Sugar()

	return l
}

func logerEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder // 修改时间编码器

	// 在日志文件中使用大写字母记录日志级别
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// NewConsoleEncoder 打印更符合人们观察的方式
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func logerWriter(logpath string, rps []int) zapcore.WriteSyncer {
	M := [4]int{10, 5, 30, 0}

	for i, rp := range rps {
		M[i] = rp
	}

	lumberJackLogger := &lumberjack.Logger{
		Filename:   logpath,
		MaxSize:    M[0],
		MaxBackups: M[1],
		MaxAge:     M[2],
		Compress:   M[3] == 1,
	}
	return zapcore.AddSync(lumberJackLogger)
}
