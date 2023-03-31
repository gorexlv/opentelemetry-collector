// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loggingexporter // import "go.opentelemetry.io/collector/exporter/loggingexporter"

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/loggingexporter/internal/otlptext"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	// The value of "type" key in configuration.
	typeStr                   = "logging"
	defaultSamplingInitial    = 2
	defaultSamplingThereafter = 500
)

var onceWarnLogLevel sync.Once

// FactoryOption applies changes to loggingExporterFactory.
type FactoryOption func(factory *loggingExporterFactory)

// WithTracesMarshaler adds tracesMarshaler.
func WithTracesMarshaler(marshaler ptrace.Marshaler) FactoryOption {
	return func(factory *loggingExporterFactory) {
		factory.tracesMarshaler = marshaler
	}
}

// WithMetricsMarshaler adds additional metric marshalers to the exporter factory.
func WithMetricsMarshaler(marshaler pmetric.Marshaler) FactoryOption {
	return func(factory *loggingExporterFactory) {
		factory.metricsMarshaler = marshaler
	}
}

// WithLogMarshaler adds additional log marshalers to the exporter factory.
func WithLogsMarshaler(marshaler plog.Marshaler) FactoryOption {
	return func(factory *loggingExporterFactory) {
		factory.logsMarshaler = marshaler
	}
}

type loggingExporterFactory struct {
	logsMarshaler    plog.Marshaler
	metricsMarshaler pmetric.Marshaler
	tracesMarshaler  ptrace.Marshaler
}

// NewFactory creates a factory for Logging exporter
func NewFactory(options ...FactoryOption) exporter.Factory {
	f := &loggingExporterFactory{
		logsMarshaler:    otlptext.NewTextLogsMarshaler(),
		metricsMarshaler: otlptext.NewTextMetricsMarshaler(),
		tracesMarshaler:  otlptext.NewTextTracesMarshaler(),
	}
	for _, o := range options {
		o(f)
	}
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(f.createTracesExporter, component.StabilityLevelDevelopment),
		exporter.WithMetrics(f.createMetricsExporter, component.StabilityLevelDevelopment),
		exporter.WithLogs(f.createLogsExporter, component.StabilityLevelDevelopment),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		LogLevel:           zapcore.InfoLevel,
		Verbosity:          configtelemetry.LevelNormal,
		SamplingInitial:    defaultSamplingInitial,
		SamplingThereafter: defaultSamplingThereafter,
	}
}

func (f *loggingExporterFactory) createTracesExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Traces, error) {
	cfg := config.(*Config)
	exporterLogger := createLogger(cfg, set.TelemetrySettings.Logger)
	s := newLoggingExporter(exporterLogger, cfg.Verbosity)
	s.logsMarshaler = f.logsMarshaler
	s.metricsMarshaler = f.metricsMarshaler
	s.tracesMarshaler = f.tracesMarshaler
	return exporterhelper.NewTracesExporter(ctx, set, cfg,
		s.pushTraces,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// Disable Timeout/RetryOnFailure and SendingQueue
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
		exporterhelper.WithShutdown(loggerSync(exporterLogger)),
	)
}

func (f *loggingExporterFactory) createMetricsExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Metrics, error) {
	cfg := config.(*Config)
	exporterLogger := createLogger(cfg, set.TelemetrySettings.Logger)
	s := newLoggingExporter(exporterLogger, cfg.Verbosity)
	s.logsMarshaler = f.logsMarshaler
	s.metricsMarshaler = f.metricsMarshaler
	s.tracesMarshaler = f.tracesMarshaler
	return exporterhelper.NewMetricsExporter(ctx, set, cfg,
		s.pushMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// Disable Timeout/RetryOnFailure and SendingQueue
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
		exporterhelper.WithShutdown(loggerSync(exporterLogger)),
	)
}

func (f *loggingExporterFactory) createLogsExporter(ctx context.Context, set exporter.CreateSettings, config component.Config) (exporter.Logs, error) {
	cfg := config.(*Config)
	exporterLogger := createLogger(cfg, set.TelemetrySettings.Logger)
	s := newLoggingExporter(exporterLogger, cfg.Verbosity)
	s.logsMarshaler = f.logsMarshaler
	s.metricsMarshaler = f.metricsMarshaler
	s.tracesMarshaler = f.tracesMarshaler
	return exporterhelper.NewLogsExporter(ctx, set, cfg,
		s.pushLogs,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// Disable Timeout/RetryOnFailure and SendingQueue
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
		exporterhelper.WithShutdown(loggerSync(exporterLogger)),
	)
}

func createLogger(cfg *Config, logger *zap.Logger) *zap.Logger {
	if cfg.warnLogLevel {
		onceWarnLogLevel.Do(func() {
			logger.Warn(
				"'loglevel' option is deprecated in favor of 'verbosity'. Set 'verbosity' to equivalent value to preserve behavior.",
				zap.Stringer("loglevel", cfg.LogLevel),
				zap.Stringer("equivalent verbosity level", cfg.Verbosity),
			)
		})
	}

	core := zapcore.NewSamplerWithOptions(
		logger.Core(),
		1*time.Second,
		cfg.SamplingInitial,
		cfg.SamplingThereafter,
	)

	return zap.New(core)
}
