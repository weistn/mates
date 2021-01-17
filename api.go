package main

import (
	"net/url"
)

// Options passed to the compiler
type Options struct {
	BaseURL *url.URL
}
