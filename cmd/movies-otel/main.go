package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/dannyrandall/movies/internal/handlers"
	"go.opentelemetry.io/contrib/detectors/aws/ecs"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
)

func main() {
	moviesTable, ok := os.LookupEnv("MOVIES_NAME")
	if !ok {
		log.Fatalf("MOVIES_NAME is not set")
	}
	log.Printf("Using %q as the movies table", moviesTable)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	otelSetup(ctx)

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load aws config: %s", err)
	}

	otelaws.AppendMiddlewares(&cfg.APIOptions)
	ddb := dynamodb.NewFromConfig(cfg)

	movieHandler := &handlers.Movie{
		Dynamo:      ddb,
		MoviesTable: moviesTable,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	mux.Handle("/api/otel/movie", otelhttp.NewHandler(movieHandler, "movie"))

	log.Printf("Starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("error serving: %s", err)
	}
}

func otelSetup(ctx context.Context) {
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithDialOption(grpc.WithBlock()))
	if err != nil {
		log.Fatalf("unable to create otel trace exporter: %v", err)
	}

	ecsResource, err := ecs.NewResourceDetector().Detect(ctx)
	if err != nil {
		log.Fatalf("%s: %v", "unable to create new OTLP trace exporter", err)
	}

	/*
		r, err := resource.Merge(
			ecsResource,
			resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceNameKey.String(copilot.ServiceName())),
		)
	*/

	idg := xray.NewIDGenerator()
	tp := trace.NewTracerProvider(
		trace.WithResource(ecsResource),
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithBatcher(exporter),
		trace.WithIDGenerator(idg),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(xray.Propagator{})
}
