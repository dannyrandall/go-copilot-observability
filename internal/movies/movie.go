package movies

import "github.com/segmentio/ksuid"

type Movie struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

func NewID() string {
	return ksuid.New().String()
}
