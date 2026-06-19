package domain

import "errors"

type Page struct {
	ID    int
	Title string
	URL   string
	HTML  []byte
}

var ErrPageNotFound = errors.New("page not found")
