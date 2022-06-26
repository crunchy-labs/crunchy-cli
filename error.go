package crunchyroll

import (
	"fmt"
	"net/http"
)

type RequestError struct {
	error

	Response *http.Response
	Message  string
}

func (re *RequestError) Error() string {
	return fmt.Sprintf("error for endpoint %s (%d): %s", re.Response.Request.URL.String(), re.Response.StatusCode, re.Message)
}
