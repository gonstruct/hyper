package hyper_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/gonstruct/hyper"
)

func TestWithBody(t *testing.T) {
	t.Run("WithBody with map[string]any", func(t *testing.T) {
		t.Parallel()

		hyper.Fake(map[string]func(req *http.Request) (*http.Response, error){
			"GET http://example.com/catch": func(req *http.Request) (*http.Response, error) {
				// Check the request body
				// bodyBytes, _ := io.ReadAll(req.Body)
				// expectedBody := `{"key1":"value1","key2":2}`
				// if string(bodyBytes) != expectedBody {
				// 	t.Errorf("Expected body %s, got %s", expectedBody, string(bodyBytes))
				// }

				// Return a mock response
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"status":"ok"}`))),
					Header:     make(http.Header),
				}, nil
			},
		})

		// faker.Assert()

		request := hyper.NewRequest(
			hyper.WithBody(map[string]any{"key1": "value1", "key2": 2}),
		)

		response := request.Get("http://example.com/catch")
		if response.R.StatusCode != 200 {
			t.Errorf("Expected status code 200, got %d", response.R.StatusCode)
		}
	})

	t.Run("WithBody with map[string]any", func(t *testing.T) {
		t.Parallel()

		hyper.Fake(map[string]func(req *http.Request) (*http.Response, error){
			"GET http://example.com/catch": func(req *http.Request) (*http.Response, error) {
				// Check the request body
				// bodyBytes, _ := io.ReadAll(req.Body)
				// expectedBody := `{"key1":"value1","key2":2}`
				// if string(bodyBytes) != expectedBody {
				// 	t.Errorf("Expected body %s, got %s", expectedBody, string(bodyBytes))
				// }

				// Return a mock response
				return &http.Response{
					StatusCode: 201,
					Body:       io.NopCloser(bytes.NewReader([]byte(`{"status":"ok"}`))),
					Header:     make(http.Header),
				}, nil
			},
		})

		// faker.Assert()

		request := hyper.NewRequest(
			hyper.WithBody(map[string]any{"key1": "value1", "key2": 2}),
		)

		response := request.Get("http://example.com/catch")
		if response.R.StatusCode != 201 {
			t.Errorf("Expected status code 201, got %d", response.R.StatusCode)
		}
	})
}
