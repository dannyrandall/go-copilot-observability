package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/dannyrandall/movies/internal/copilot"
	"github.com/dannyrandall/movies/internal/otel"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	otelotel "go.opentelemetry.io/otel"
)

func main() {
	svcName := copilot.ServiceName("movies-processor")
	if err := otel.SetupTracer(context.Background(), svcName); err != nil {
		log.Fatalf("unable to setup otel tracer: %s", err)
	}

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("unable to load aws config: %s", err)
	}

	otelaws.AppendMiddlewares(&cfg.APIOptions)

	q := &MovieQueue{
		SQS:            sqs.NewFromConfig(cfg),
		HTTP:           otelhttp.DefaultClient,
		Tracer:         otelotel.Tracer(""),
		QueueName:      fmt.Sprintf("%s-%s-createMovie", copilot.App(), copilot.Environment()),
		QueueURL:       copilot.QueueURI(),
		CreateMovieURL: fmt.Sprintf("http://movies-backend-service.%s.%s.local:8080/movies/api/movie", copilot.Environment(), copilot.App()),
	}

	log.Printf("Waiting for events from %s", q.QueueURL)

	if err := q.ReceiveAndProcess(context.Background()); err != nil {
		log.Fatalf("unable to receive and process: %s", err)
	}
}
