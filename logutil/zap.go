package logutil

import (
	// "github.com/natefinch/lumberjack"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Log struct {
	Logger   *zap.Logger
	Logpath  string
	Loglevel string
}

func getEncode() zapcore.Encoder {
	encoderconfig := zap.NewProductionEncoderConfig()
	encoderconfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderconfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zapcore.NewConsoleEncoder(encoderconfig)

}

// func getLoggerWriter() zapcore.WriteSyncer{
// 	file,err := os.OpenFile("../logs/prome.log",os.O_WRONLY|os.O_APPEND|os.O_CREATE,0644)
// 	if err != nil {
// 		fmt.Println("log file open error",err)
// 	}
// 	defer file.Close()
// 	return zapcore.AddSync(file)
// }

// func getLoggerWriter() zapcore.WriteSyncer {
// 	lumberJackLogger := &lumberjack.Logger{
// 		FIlename:   "../logs/prome.log",
// 		MaxSize:    10,
// 		MaxBackups: 5,
// 		MaxAge:     30,
// 		Compress:   false,
// 	}
// 	return zapcore.AddSync(lumberJackLogger)

// }

func Logger(logpath string, loglevel string) *zap.Logger {
	lumber := lumberjack.Logger{
		Filename:   logpath,
		MaxSize:    200,
		MaxBackups: 30,
		MaxAge:     30,
		Compress:   false,
	}
	write := zapcore.AddSync(&lumber)

	var level zapcore.Level
	switch loglevel {
	case "debug":
		level = zap.DebugLevel
	case "info":
		level = zap.InfoLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	default:
		level = zap.InfoLevel
	}

	encoderconfig := zapcore.EncoderConfig{
		TimeKey:        "time",
		LevelKey:       "level",
		NameKey:        "logger",
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.FullCallerEncoder,
		EncodeName:     zapcore.FullNameEncoder,
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderconfig),
		write,
		level,
	)
	logger := zap.New(core, zap.AddCaller())
	return logger

}
