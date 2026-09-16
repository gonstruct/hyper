package hyper

type requestOption func(*Request)

func WithBase(base string) requestOption {
	return func(request *Request) {
		request.base = base
	}
}

func WithHeaders[M H | headerOption](mapOrOptions ...M) requestOption {
	return func(request *Request) {
		if request.headers == nil {
			request.headers = newHeaders()
		}

		for _, item := range mapOrOptions {
			switch v := any(item).(type) {
			case H:
				for _, option := range v.toHeaderOptions() {
					option(request.headers)
				}
			case headerOption:
				v(request.headers)
			}
		}
	}
}

func WithQueryParams[M Q | queryParamOption](mapOrOptions ...M) requestOption {
	return func(request *Request) {
		if request.queryParams == nil {
			request.queryParams = newQueryParams()
		}

		for _, item := range mapOrOptions {
			switch v := any(item).(type) {
			case Q:
				for _, option := range v.toQueryParamOptions() {
					option(request.queryParams)
				}
			case queryParamOption:
				v(request.queryParams)
			}
		}
	}
}
