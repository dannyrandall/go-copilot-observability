package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/dannyrandall/movies/internal/movies"
)

type Movie struct {
	Dynamo      *dynamodb.DynamoDB
	MoviesTable string
}

func (m *Movie) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		m.getMovie(w, r)
	case http.MethodPost:
		m.createMovie(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (m *Movie) getMovie(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	id := r.URL.Query().Get("id")
	log.Printf("Getting movie %q", id)

	result, err := m.Dynamo.GetItemWithContext(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(m.MoviesTable),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {
				S: aws.String(id),
			},
		},
	})
	switch {
	case err != nil:
		http.Error(w, fmt.Sprintf("get item: %s", err), http.StatusInternalServerError)
		return
	case result.Item == nil:
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var movie movies.Movie
	if err := dynamodbattribute.UnmarshalMap(result.Item, &movie); err != nil {
		http.Error(w, fmt.Sprintf("unmarshal result: %s", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Got movie %+v", movie)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(movie); err != nil {
		log.Printf("error encoding movie %q: %s", movie.ID, err)
	}
}

func (m *Movie) createMovie(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	dec := json.NewDecoder(r.Body)

	var movie movies.Movie
	if err := dec.Decode(&movie); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	movie.ID = movies.NewID()
	log.Printf("Creating movie %+v", movie)

	av, err := dynamodbattribute.MarshalMap(movie)
	if err != nil {
		http.Error(w, fmt.Sprintf("marshal movie: %s", err), http.StatusBadRequest)
		return
	}

	input := &dynamodb.PutItemInput{
		Item:      av,
		TableName: aws.String(m.MoviesTable),
	}

	_, err = m.Dynamo.PutItemWithContext(ctx, input)
	if err != nil {
		http.Error(w, fmt.Sprintf("put item: %s", err), http.StatusBadRequest)
		return
	}

	log.Printf("Created movie %q", movie.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(movie); err != nil {
		log.Printf("error encoding movie %q: %s", movie.ID, err)
	}
}
