// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transformprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor"

import (
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/logs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/traces"
)

// Config defines the configuration for the processor.
type Config struct {
	// ErrorMode determines how the processor reacts to errors that occur while processing a statement.
	// Valid values are `ignore` and `propagate`.
	// `ignore` means the processor ignores errors returned by statements and continues on to the next statement. This is the recommended mode.
	// `propagate` means the processor returns the error up the pipeline.  This will result in the payload being dropped from the collector.
	// The default value is `propagate`.
	ErrorMode ottl.ErrorMode `mapstructure:"error_mode"`

	TraceStatements  []common.ContextStatements `mapstructure:"trace_statements"`
	MetricStatements []common.ContextStatements `mapstructure:"metric_statements"`
	LogStatements    []common.ContextStatements `mapstructure:"log_statements"`
}

var _ component.Config = (*Config)(nil)

func (c *Config) Validate() error {
	if len(c.TraceStatements) > 0 {
		pc, err := common.NewTraceParserCollection(component.TelemetrySettings{Logger: zap.NewNop()}, common.WithSpanParser(traces.SpanFunctions()), common.WithSpanEventParser(traces.SpanEventFunctions()))
		if err != nil {
			return err
		}
		for _, cs := range c.TraceStatements {
			_, err = pc.ParseContextStatements(cs)
			if err != nil {
				return err
			}
		}
	}

	if len(c.MetricStatements) > 0 {
		pc, err := common.NewMetricParserCollection(component.TelemetrySettings{Logger: zap.NewNop()}, common.WithMetricParser(metrics.MetricFunctions()), common.WithDataPointParser(metrics.DataPointFunctions()))
		if err != nil {
			return err
		}
		for _, cs := range c.MetricStatements {
			_, err = pc.ParseContextStatements(cs)
			if err != nil {
				return err
			}
		}
	}

	if len(c.LogStatements) > 0 {
		pc, err := common.NewLogParserCollection(component.TelemetrySettings{Logger: zap.NewNop()}, common.WithLogParser(logs.LogFunctions()))
		if err != nil {
			return err
		}
		for _, cs := range c.LogStatements {
			_, err = pc.ParseContextStatements(cs)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
