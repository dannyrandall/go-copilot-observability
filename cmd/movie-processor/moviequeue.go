package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/dannyrandall/movies/internal/movies"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

type MovieQueue struct {
	CreateMovieURL string
	HTTP           *http.Client
	Tracer         trace.Tracer

	sqs       *sqs.Client
	queueName string
	queueURL  string
}

func NewMovieQueue(ctx context.Context, cfg aws.Config, queueName string) (*MovieQueue, error) {
	q := &MovieQueue{
		queueName: queueName,
		sqs:       sqs.NewFromConfig(cfg),
		HTTP:      otelhttp.DefaultClient,
		Tracer:    otel.Tracer(""),
	}

	res, err := q.sqs.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(queueName),
	})
	if err != nil {
		return nil, fmt.Errorf("get queue url: %w", err)
	}

	q.queueURL = aws.ToString(res.QueueUrl)
	return q, nil
}

func (q *MovieQueue) ReceiveAndProcess(ctx context.Context) error {
	for {
		msgs, err := q.recieveMessages(ctx)
		if err != nil {
			return fmt.Errorf("receive messages: %w", err)
		}

		for _, msg := range msgs {
			if err := q.processMessage(ctx, msg); err != nil {
				log.Printf("unable to process message %q: %s", aws.ToString(msg.MessageId), err)
			}
		}
	}
}

func (q *MovieQueue) recieveMessages(ctx context.Context) ([]types.Message, error) {
	res, err := q.sqs.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.queueURL),
		MaxNumberOfMessages: 1,
	})
	if err != nil {
		return nil, err
	}

	return res.Messages, nil
}

func (q *MovieQueue) processMessage(ctx context.Context, msg types.Message) error {
	ctx, span := q.Tracer.Start(ctx, "process-movie",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(semconv.MessagingSystemKey.String("AmazonSQS")),
		trace.WithAttributes(semconv.MessagingDestinationKey.String(q.queueName)),
		trace.WithAttributes(semconv.MessagingDestinationKindQueue),
		trace.WithAttributes(semconv.MessagingMessageIDKey.String(aws.ToString(msg.MessageId))))
	defer span.End()

	var movie movies.Movie

	if err := q.createMovie(ctx, movie); err != nil {
		return fmt.Errorf("create movie: %w", err)
	}

	if err := q.deleteMessage(ctx, msg.ReceiptHandle); err != nil {
		return fmt.Errorf("delete message: %w", err)
	}

	return nil
}

func (q *MovieQueue) createMovie(ctx context.Context, movie movies.Movie) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, q.CreateMovieURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := q.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("bad response: status code %v", resp.StatusCode)
	}

	return nil
}

func (q *MovieQueue) deleteMessage(ctx context.Context, receiptHandle *string) error {
	_, err := q.sqs.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: receiptHandle,
	})

	return err
}
