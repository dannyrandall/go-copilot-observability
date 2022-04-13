package movies

import "github.com/segmentio/ksuid"

type Movie struct {
	ID    string `json:"id" dynamodbav:"id"`
	Title string `json:"title" dynamodbav:"title"`
	Year  int    `json:"year" dynamodbav:"year"`
}

func NewID() string {
	return ksuid.New().String()
}
