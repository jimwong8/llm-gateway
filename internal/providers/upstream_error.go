package providers

import "fmt"

type upstreamHTTPError struct {
	statusCode int
	message    string
	headers    map[string]string
}

func (e upstreamHTTPError) Error() string {
	if e.message != "" {
		return fmt.Sprintf("upstream http %d: %s", e.statusCode, e.message)
	}
	return fmt.Sprintf("upstream http %d", e.statusCode)
}

func (e upstreamHTTPError) HTTPStatusCode() int {
	return e.statusCode
}

func (e upstreamHTTPError) HTTPResponseHeaders() map[string]string {
	return e.headers
}

func newUpstreamHTTPError(statusCode int, message string) upstreamHTTPError {
	return upstreamHTTPError{statusCode: statusCode, message: message, headers: map[string]string{}}
}

func newUpstreamHTTPErrorWithHeaders(statusCode int, message string, headers map[string]string) upstreamHTTPError {
	return upstreamHTTPError{statusCode: statusCode, message: message, headers: headers}
}
