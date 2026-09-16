package hyper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
)

type body interface {
	Read() io.Reader
}

func WithBody[T any | url.Values | string | []byte | map[string]any](data T) requestOption {
	return func(r *Request) {
		switch value := any(data).(type) {
		case url.Values:
			r.body = formBody{data: value}
		case string:
			r.body = stringBody{data: value}
		case []byte:
			r.body = bytesBody{data: value}
		case map[string]any:
			values := make(url.Values, len(value))
			for k, v := range value {
				values.Add(k, fmt.Sprint(v))
			}
			r.body = formBody{data: values}
		default:
			r.body = marshalBody{data: value}
		}
	}
}

type marshalBody struct{ data any }

func (m marshalBody) Read() io.Reader {
	marshalled, err := json.Marshal(m.data)
	if err != nil {
		return nil
	}

	return bytes.NewReader(marshalled)
}

type formBody struct{ data url.Values }

func (m formBody) Read() io.Reader {
	return bytes.NewReader([]byte(m.data.Encode()))
}

type stringBody struct{ data string }

func (s stringBody) Read() io.Reader {
	return bytes.NewReader([]byte(s.data))
}

type bytesBody struct{ data []byte }

func (b bytesBody) Read() io.Reader {
	return bytes.NewReader(b.data)
}

type readerBody struct{ reader io.Reader }

func (r readerBody) Read() io.Reader {
	return r.reader
}

func WithReaderBody(reader io.Reader) requestOption {
	return func(r *Request) {
		r.body = readerBody{reader: reader}
	}
}
