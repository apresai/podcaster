package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"go.opentelemetry.io/otel/trace"
)

// InitLogger creates a structured JSON logger that writes to stderr.
// On AgentCore (detected via OTEL_EXPORTER_OTLP_LOGS_HEADERS), it also
// ships log events to CloudWatch Logs using the SDK.
func InitLogger() *slog.Logger {
	stderrHandler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	var handler slog.Handler = &traceHandler{inner: stderrHandler}

	// If running on AgentCore, also write to CloudWatch
	logHeaders := os.Getenv("OTEL_EXPORTER_OTLP_LOGS_HEADERS")
	if logHeaders != "" {
		logGroup, logStream := parseLogHeaders(logHeaders)
		if logGroup != "" && logStream != "" {
			cwHandler, err := newCloudWatchHandler(logGroup, logStream)
			if err != nil {
				fmt.Fprintf(os.Stderr, "WARN: failed to init CloudWatch logger: %v\n", err)
			} else {
				handler = &multiHandler{
					handlers: []slog.Handler{
						&traceHandler{inner: stderrHandler},
						&traceHandler{inner: cwHandler},
					},
				}
			}
		}
	}

	return slog.New(handler)
}

// parseLogHeaders extracts log group and stream from OTEL_EXPORTER_OTLP_LOGS_HEADERS.
// Format: "x-aws-log-group=/aws/.../foo,x-aws-log-stream=otel-rt-logs,..."
func parseLogHeaders(headers string) (logGroup, logStream string) {
	for _, part := range strings.Split(headers, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "x-aws-log-group":
			logGroup = kv[1]
		case "x-aws-log-stream":
			logStream = kv[1]
		}
	}
	return
}

// cloudWatchHandler is a slog.Handler that writes JSON log events to CloudWatch Logs.
type cloudWatchHandler struct {
	client    *cloudwatchlogs.Client
	logGroup  string
	logStream string
	level     slog.Level

	mu     sync.Mutex
	buffer []cwltypes.InputLogEvent
	timer  *time.Timer
}

func newCloudWatchHandler(logGroup, logStream string) (*cloudWatchHandler, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := cloudwatchlogs.NewFromConfig(cfg)

	h := &cloudWatchHandler{
		client:    client,
		logGroup:  logGroup,
		logStream: logStream,
		level:     slog.LevelInfo,
	}

	return h, nil
}

func (h *cloudWatchHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *cloudWatchHandler) Handle(_ context.Context, r slog.Record) error {
	// Format as JSON
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	record := map[string]any{
		"time":  r.Time.Format(time.RFC3339Nano),
		"level": r.Level.String(),
		"msg":   r.Message,
	}
	r.Attrs(func(a slog.Attr) bool {
		record[a.Key] = a.Value.Any()
		return true
	})
	enc.Encode(record)

	event := cwltypes.InputLogEvent{
		Message:   aws.String(buf.String()),
		Timestamp: aws.Int64(r.Time.UnixMilli()),
	}

	h.mu.Lock()
	h.buffer = append(h.buffer, event)
	needsFlush := len(h.buffer) >= 25 // flush every 25 events
	if h.timer == nil {
		h.timer = time.AfterFunc(5*time.Second, h.flush)
	}
	h.mu.Unlock()

	if needsFlush {
		go h.flush()
	}
	return nil
}

func (h *cloudWatchHandler) flush() {
	h.mu.Lock()
	events := h.buffer
	h.buffer = nil
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
	h.mu.Unlock()

	if len(events) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := h.client.PutLogEvents(ctx, &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  &h.logGroup,
		LogStreamName: &h.logStream,
		LogEvents:     events,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: CloudWatch PutLogEvents failed: %v\n", err)
	}
}

func (h *cloudWatchHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For simplicity, return self (attrs are handled per-record)
	return h
}

func (h *cloudWatchHandler) WithGroup(name string) slog.Handler {
	return h
}

// traceHandler wraps a slog.Handler to inject trace_id and span_id from context.
type traceHandler struct {
	inner slog.Handler
}

func (h *traceHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{inner: h.inner.WithGroup(name)}
}

// multiHandler fans out to multiple slog handlers.
type multiHandler struct {
	handlers []slog.Handler
}

func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, r.Level) {
			handler.Handle(ctx, r)
		}
	}
	return nil
}

func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for i, handler := range h.handlers {
		handlers[i] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}
