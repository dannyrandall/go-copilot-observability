package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/dannyrandall/movies/internal/copilot"
	"go.opentelemetry.io/contrib/detectors/aws/ecs"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	otelxray "go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"
)

func main() {
	shutdown, err := otelSetup(context.Background(), copilot.ServiceName("movies-processor"))
	if err != nil {
		log.Fatalf("unable to setup open telemetry: %s", err)
	}
	defer shutdown(context.Background())

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("unable to load aws config: %s", err)
	}

	otelaws.AppendMiddlewares(&cfg.APIOptions)

	q := &MovieQueue{
		SQS:            sqs.NewFromConfig(cfg),
		HTTP:           otelhttp.DefaultClient,
		Tracer:         otel.Tracer(""),
		QueueName:      fmt.Sprintf("%s-%s-createMovie", copilot.App(), copilot.Environment()),
		QueueURL:       copilot.QueueURI(),
		CreateMovieURL: fmt.Sprintf("http://movies-backend-service.%s.%s.local:8080/movies/api/otel/movie", copilot.Environment(), copilot.App()),
	}

	log.Printf("Waiting for events from %s", q.QueueURL)

	if err := q.ReceiveAndProcess(context.Background()); err != nil {
		log.Fatalf("unable to receive and process: %s", err)
	}
}

func otelSetup(ctx context.Context, svcName string) (func(context.Context), error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithDialOption(grpc.WithBlock()))
	if err != nil {
		return nil, fmt.Errorf("create otel trace exporter: %w", err)
	}

	ecsResource, err := ecs.NewResourceDetector().Detect(ctx)
	if err != nil {
		return nil, fmt.Errorf("create ecs resource detector: %w", err)
	}

	r, err := resource.Merge(
		ecsResource,
		resource.NewWithAttributes(ecsResource.SchemaURL(), semconv.ServiceNameKey.String(svcName)),
	)
	if err != nil {
		return nil, fmt.Errorf("merge resources: %s", err)
	}

	idg := otelxray.NewIDGenerator()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(r),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithIDGenerator(idg),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(otelxray.Propagator{})

	return func(ctx context.Context) {
		_ = tp.Shutdown(ctx)
	}, nil
}
