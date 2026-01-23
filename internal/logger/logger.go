package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wraps zap.Logger with convenience methods
type Logger struct {
	*zap.Logger
}

// New creates a new logger instance
func New(level string) (*Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}
	
	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapLevel),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    productionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	
	zapLogger, err := config.Build()
	if err != nil {
		return nil, err
	}
	
	return &Logger{Logger: zapLogger}, nil
}

// NewDevelopment creates a development logger with console output
func NewDevelopment() (*Logger, error) {
	config := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    developmentEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	
	zapLogger, err := config.Build()
	if err != nil {
		return nil, err
	}
	
	return &Logger{Logger: zapLogger}, nil
}

// productionEncoderConfig returns encoder config for production
func productionEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// developmentEncoderConfig returns encoder config for development
func developmentEncoderConfig() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}
}

// WithEnvironment adds environment ID to logger context
func (l *Logger) WithEnvironment(envID string) *Logger {
	return &Logger{
		Logger: l.With(zap.String("environment_id", envID)),
	}
}

// WithUser adds user ID to logger context
func (l *Logger) WithUser(userID string) *Logger {
	return &Logger{
		Logger: l.With(zap.String("user_id", userID)),
	}
}

// WithOperation adds operation name to logger context
func (l *Logger) WithOperation(op string) *Logger {
	return &Logger{
		Logger: l.With(zap.String("operation", op)),
	}
}
