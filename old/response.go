package hyper

import "net/http"

type Response struct {
	R   *http.Response
	err error

	request *Request
}
