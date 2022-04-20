package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/dannyrandall/movies/internal/movies"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

type MovieQueue struct {
	SQS    *sqs.Client
	HTTP   *http.Client
	Tracer trace.Tracer

	CreateMovieURL string
	QueueName      string
	QueueURL       string
}

func (q *MovieQueue) ReceiveAndProcess(ctx context.Context) error {
	recvAndProcess := func(ctx context.Context) error {
		ctx, span := q.Tracer.Start(ctx, "recvAndProcess",
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(semconv.MessagingSystemKey.String("AmazonSQS")),
			trace.WithAttributes(semconv.MessagingDestinationKey.String(q.QueueName)),
			trace.WithAttributes(semconv.MessagingDestinationKindQueue))
		defer span.End()

		msgs, err := q.recieveMessages(ctx)
		if err != nil {
			return spanErrorf(span, "receive message: %w", err)
		}

		for _, msg := range msgs {
			if err := q.processMessage(ctx, msg); err != nil {
				return spanErrorf(span, "process message %q: %w", aws.ToString(msg.MessageId), err)
			}
		}

		return nil
	}

	for {
		if err := recvAndProcess(ctx); err != nil {
			log.Printf("error: %s", err)
		}
	}
}

func spanErrorf(span trace.Span, format string, a ...any) error {
	err := fmt.Errorf(format, a...)
	span.SetStatus(codes.Error, err.Error())
	return err
}

func (q *MovieQueue) recieveMessages(ctx context.Context) ([]types.Message, error) {
	res, err := q.SQS.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.QueueURL),
		MaxNumberOfMessages: 1,
		WaitTimeSeconds:     20,
	})
	if err != nil {
		return nil, err
	}

	return res.Messages, nil
}

func (q *MovieQueue) processMessage(ctx context.Context, msg types.Message) error {
	ctx, span := q.Tracer.Start(ctx, "processMessage", trace.WithAttributes(semconv.MessagingMessageIDKey.String(aws.ToString(msg.MessageId))))
	defer span.End()

	var movie movies.Movie
	if err := json.Unmarshal([]byte(aws.ToString(msg.Body)), &movie); err != nil {
		return spanErrorf(span, "unmarshal movie: %w", err)
	}

	if err := q.createMovie(ctx, movie); err != nil {
		return spanErrorf(span, "create movie: %w", err)
	}

	if err := q.deleteMessage(ctx, msg.ReceiptHandle); err != nil {
		return spanErrorf(span, "delete message: %w", err)
	}

	return nil
}

func (q *MovieQueue) createMovie(ctx context.Context, movie movies.Movie) error {
	data, err := json.Marshal(movie)
	if err != nil {
		return fmt.Errorf("encode movie: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, q.CreateMovieURL, bytes.NewBuffer(data))
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
	_, err := q.SQS.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.QueueURL),
		ReceiptHandle: receiptHandle,
	})

	return err
}
