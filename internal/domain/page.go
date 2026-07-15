package domain

import "errors"

type Page struct {
	ID    int
	Title string
	URL   string
	HTML  []byte
	Score int
}

var ErrPageNotFound = errors.New("page not found")
