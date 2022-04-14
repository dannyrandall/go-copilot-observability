package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-xray-sdk-go/instrumentation/awsv2"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/dannyrandall/movies/internal/copilot"
	"github.com/dannyrandall/movies/internal/handlers"
	"go.opentelemetry.io/contrib/detectors/aws/ecs"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	otelxray "go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"
)

func main() {
	// Get DynamoDB table name
	moviesTable, ok := os.LookupEnv("MOVIES_NAME")
	if !ok {
		log.Fatalf("MOVIES_NAME is not set")
	}
	log.Printf("Using %q as the DynamoDB movies table", moviesTable)

	// Timeout for setup functions
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Load AWS SDK config
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("unable to load aws config: %s", err)
	}

	svcName := copilot.ServiceName("movies")
	mux := http.NewServeMux()

	// simple load balancer health check endpoint
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	// Open Telemetry SDK & endpoint setup
	otelAWSCfg := cfg.Copy()
	otelErr := otelSetup(ctx, svcName)
	if otelErr != nil {
		log.Printf("error setting up open telemetry: %s", otelErr)
		log.Printf("OTEL endpoint will return a 500.")
		mux.HandleFunc("/movies/api/otel/movie", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "error setting up open telemetry: %s", otelErr)
		})
	} else {
		otelaws.AppendMiddlewares(&otelAWSCfg.APIOptions)
		mux.Handle("/movies/api/otel/movie", otelhttp.NewHandler(&handlers.Movie{
			Dynamo:      dynamodb.NewFromConfig(otelAWSCfg),
			MoviesTable: moviesTable,
		}, "movie"))
	}

	// X-Ray SDK & endpoint setup
	xrayAWSCfg := cfg.Copy()
	awsv2.AWSV2Instrumentor(&xrayAWSCfg.APIOptions)
	mux.Handle("/movies/api/xray/movie", xray.Handler(xray.NewFixedSegmentNamer(svcName), &handlers.Movie{
		Dynamo:      dynamodb.NewFromConfig(xrayAWSCfg),
		MoviesTable: moviesTable,
	}))

	// Run HTTP Server
	log.Printf("Starting server on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatalf("error serving: %s", err)
	}
}

func otelSetup(ctx context.Context, svcName string) error {
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithDialOption(grpc.WithBlock()))
	if err != nil {
		return fmt.Errorf("create otel trace exporter: %w", err)
	}

	ecsResource, err := ecs.NewResourceDetector().Detect(ctx)
	if err != nil {
		return fmt.Errorf("create ecs resource detector: %w", err)
	}

	r, err := resource.Merge(
		ecsResource,
		resource.NewWithAttributes(ecsResource.SchemaURL(), semconv.ServiceNameKey.String(svcName)),
	)
	if err != nil {
		return fmt.Errorf("merge resources: %s", err)
	}

	idg := otelxray.NewIDGenerator()
	tp := trace.NewTracerProvider(
		trace.WithResource(r),
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithBatcher(exporter),
		trace.WithIDGenerator(idg),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(otelxray.Propagator{})
	return nil
}
