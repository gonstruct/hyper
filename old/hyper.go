package hyper

import "net/http"

// func main() {
// 	response := Get("/myUrl/{id}",
// 		WithBody(map[string]any{"name": "value"}),
// 	)
// }

func (request *Request) Get(url string, options ...requestOption) *Response {
	request.method = http.MethodGet
	request.url = url
	return request.apply(options...).do()
}

func Get(url string, options ...requestOption) *Response {
	return NewRequest(options...).Get(url)
}
