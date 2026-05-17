package providers

import "fmt"

type upstreamHTTPError struct {
	statusCode int
	message    string
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

func newUpstreamHTTPError(statusCode int, message string) upstreamHTTPError {
	return upstreamHTTPError{statusCode: statusCode, message: message}
}
