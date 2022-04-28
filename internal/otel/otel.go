package otel

import (
	"context"
	"fmt"
	"time"

	otelxray "go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
)

func SetupTracer(ctx context.Context, svcName string) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithDialOption(grpc.WithBlock()))
	if err != nil {
		return fmt.Errorf("create otel trace exporter: %w", err)
	}

	r := resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceNameKey.String(svcName))
	idg := otelxray.NewIDGenerator()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithIDGenerator(idg),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(otelxray.Propagator{})
	return nil
}

func XRayTraceID(span trace.Span) string {
	id := span.SpanContext().TraceID().String()
	if len(id) < 9 {
		return id
	}

	return fmt.Sprintf("1-%s-%s", id[:8], id[8:])
}
