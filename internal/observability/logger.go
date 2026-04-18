package observability

import (
	"fmt"
	"strings"

	"github.com/avf/avf-vending-api/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// NewLogger builds a production-ready zap logger from configuration.
func NewLogger(cfg *config.Config) (*zap.Logger, error) {
	if cfg == nil {
		return nil, fmt.Errorf("observability: nil config")
	}

	level, err := parseZapLevel(cfg.LogLevel)
	if err != nil {
		return nil, err
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encCfg.EncodeDuration = zapcore.StringDurationEncoder

	encoding := "json"
	if strings.EqualFold(cfg.LogFormat, "text") {
		encoding = "console"
	}

	zapCfg := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       cfg.AppEnv == config.AppEnvDevelopment,
		DisableCaller:     false,
		DisableStacktrace: false,
		Sampling:          nil,
		Encoding:          encoding,
		EncoderConfig:     encCfg,
		OutputPaths:       []string{"stderr"},
		ErrorOutputPaths:  []string{"stderr"},
		InitialFields: map[string]any{
			"app_env": string(cfg.AppEnv),
		},
	}

	logger, err := zapCfg.Build(zap.AddStacktrace(zapcore.ErrorLevel))
	if err != nil {
		return nil, fmt.Errorf("observability: build logger: %w", err)
	}
	return logger, nil
}

func parseZapLevel(raw string) (zapcore.Level, error) {
	var l zapcore.Level
	if err := l.Set(strings.TrimSpace(strings.ToLower(raw))); err != nil {
		return zapcore.InfoLevel, fmt.Errorf("observability: invalid LOG_LEVEL %q", raw)
	}
	return l, nil
}
