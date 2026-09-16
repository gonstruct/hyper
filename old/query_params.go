package hyper

import (
	"fmt"
	"net/url"
)

func newQueryParams() *queryParams {
	return &queryParams{url.Values{}}
}

type queryParams struct {
	url.Values
}

type (
	queryParamOption func(*queryParams)
	Q                map[string]any
)

func WithQueryParam(key string, value ...any) queryParamOption {
	return func(q *queryParams) {
		for _, v := range value {
			q.Add(key, fmt.Sprint(v))
		}
	}
}

func (q Q) toQueryParamOptions() []queryParamOption {
	options := make([]queryParamOption, 0, len(q))
	for key, value := range q {
		options = append(options, WithQueryParam(key, value))
	}
	return options
}
