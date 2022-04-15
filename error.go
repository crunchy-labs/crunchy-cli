package crunchyroll

import "fmt"

// AccessError is an error which will be returned when some special sort of api request fails.
// See Crunchyroll.request when the error gets used.
type AccessError struct {
	error

	URL     string
	Body    []byte
	Message string
}

func (ae *AccessError) Error() string {
	if ae.Message == "" {
		return fmt.Sprintf("Access token invalid for url %s\nBody: %s", ae.URL, string(ae.Body))
	} else {
		return ae.Message
	}
}
