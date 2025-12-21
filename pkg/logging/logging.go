// Package logging provides structured logging capabilities for the application.
// It wraps uber-go/zap to provide a consistent logging interface with support
// for different log levels, output formats, and contextual fields.
package logging

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	globalLogger *zap.SugaredLogger
	globalLevel  zap.AtomicLevel
	once         sync.Once
	mu           sync.RWMutex
)

// Config holds logging configuration options.
type Config struct {
	// Level is the minimum log level (debug, info, warn, error).
	Level string `yaml:"level"`

	// Format is the output format (json, console).
	Format string `yaml:"format"`

	// OutputPaths are the destinations for log output.
	// Defaults to stderr if not specified.
	OutputPaths []string `yaml:"output_paths"`

	// Development enables development mode with more verbose output.
	Development bool `yaml:"development"`

	// DisableCaller disables caller information in log output.
	DisableCaller bool `yaml:"disable_caller"`

	// DisableStacktrace disables stacktrace for error logs.
	DisableStacktrace bool `yaml:"disable_stacktrace"`
}

// DefaultConfig returns a sensible default logging configuration.
func DefaultConfig() Config {
	return Config{
		Level:       "info",
		Format:      "console",
		OutputPaths: []string{"stderr"},
		Development: false,
	}
}

// Init initializes the global logger with the given configuration.
// This should be called once at application startup.
// It is safe to call multiple times; subsequent calls are no-ops.
func Init(cfg Config) error {
	var initErr error
	once.Do(func() {
		initErr = initLogger(cfg)
	})
	return initErr
}

// MustInit initializes the logger and panics on error.
func MustInit(cfg Config) {
	if err := Init(cfg); err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
}

// Reinit forces reinitialization of the logger.
// This is primarily useful for testing.
func Reinit(cfg Config) error {
	mu.Lock()
	defer mu.Unlock()
	return initLogger(cfg)
}

func initLogger(cfg Config) error {
	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Create atomic level for runtime changes
	globalLevel = zap.NewAtomicLevelAt(level)

	// Build encoder config based on format
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Customize for console output
	if cfg.Format == "console" {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeDuration = zapcore.StringDurationEncoder
	}

	// Set output paths
	outputPaths := cfg.OutputPaths
	if len(outputPaths) == 0 {
		outputPaths = []string{"stderr"}
	}

	// Build zap config
	zapConfig := zap.Config{
		Level:             globalLevel,
		Development:       cfg.Development,
		DisableCaller:     cfg.DisableCaller,
		DisableStacktrace: cfg.DisableStacktrace,
		Encoding:          cfg.Format,
		EncoderConfig:     encoderConfig,
		OutputPaths:       outputPaths,
		ErrorOutputPaths:  []string{"stderr"},
	}

	// Handle console encoding
	if cfg.Format == "console" {
		zapConfig.Encoding = "console"
	}

	// Build logger
	logger, err := zapConfig.Build(
		zap.AddCallerSkip(1), // Skip the logging wrapper functions
	)
	if err != nil {
		return err
	}

	globalLogger = logger.Sugar()
	return nil
}

// ensureInitialized makes sure the logger is initialized with defaults.
func ensureInitialized() {
	if globalLogger == nil {
		_ = Init(DefaultConfig())
	}
}

// Logger returns the global logger instance.
func Logger() *zap.SugaredLogger {
	mu.RLock()
	defer mu.RUnlock()
	ensureInitialized()
	return globalLogger
}

// SetLevel changes the log level at runtime.
func SetLevel(level string) error {
	lvl, err := zapcore.ParseLevel(level)
	if err != nil {
		return err
	}
	globalLevel.SetLevel(lvl)
	return nil
}

// GetLevel returns the current log level.
func GetLevel() string {
	return globalLevel.Level().String()
}

// WithFields returns a logger with the given fields attached.
func WithFields(keysAndValues ...interface{}) *zap.SugaredLogger {
	mu.RLock()
	defer mu.RUnlock()
	ensureInitialized()
	return globalLogger.With(keysAndValues...)
}

// WithComponent returns a logger for a specific component.
func WithComponent(component string) *zap.SugaredLogger {
	return WithFields("component", component)
}

// Debug logs a debug message with optional key-value pairs.
func Debug(msg string, keysAndValues ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	ensureInitialized()
	globalLogger.Debugw(msg, keysAndValues...)
}

// Info logs an info message with optional key-value pairs.
func Info(msg string, keysAndValues ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	ensureInitialized()
	globalLogger.Infow(msg, keysAndValues...)
}

// Warn logs a warning message with optional key-value pairs.
func Warn(msg string, keysAndValues ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	ensureInitialized()
	globalLogger.Warnw(msg, keysAndValues...)
}

// Error logs an error message with optional key-value pairs.
func Error(msg string, keysAndValues ...interface{}) {
	mu.RLock()
	defer mu.RUnlock()
	ensureInitialized()
	globalLogger.Errorw(msg, keysAndValues...)
}

// Fatal logs a fatal message and exits the application.
func Fatal(msg string, keysAndValues ...interface{}) {
	mu.RLock()
	logger := globalLogger
	mu.RUnlock()
	ensureInitialized()
	logger.Fatalw(msg, keysAndValues...)
	os.Exit(1)
}

// Sync flushes any buffered log entries.
// This should be called before the application exits.
func Sync() error {
	mu.RLock()
	defer mu.RUnlock()
	if globalLogger != nil {
		return globalLogger.Sync()
	}
	return nil
}

// NewNopLogger returns a logger that discards all output.
// Useful for testing.
func NewNopLogger() *zap.SugaredLogger {
	return zap.NewNop().Sugar()
}

// NewTestLogger returns a logger suitable for testing.
// It logs to the test output at debug level.
func NewTestLogger() *zap.SugaredLogger {
	config := zap.NewDevelopmentConfig()
	config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	logger, _ := config.Build()
	return logger.Sugar()
}
