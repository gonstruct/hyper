package hyper

import (
	"fmt"
	"net/http"
)

func newHeaders() *headers {
	return &headers{http.Header{}}
}

type headers struct {
	http.Header
}

type (
	headerOption func(*headers)
	H            map[string]any
)

func (h H) toHeaderOptions() []headerOption {
	options := make([]headerOption, 0, len(h))
	for key, value := range h {
		options = append(options, WithHeader(key, value))
	}
	return options
}

func WithHeader(key string, value any) headerOption {
	return func(h *headers) {
		h.Add(key, fmt.Sprint(value))
	}
}
