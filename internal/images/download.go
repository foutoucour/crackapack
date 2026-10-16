package images

import (
	"fmt"
	"io"
	"net/http"
)

// statusError is a download that the server answered with a status other than 200.
type statusError struct {
	what string
	code int
}

func (e *statusError) Error() string { return fmt.Sprintf("%s: status %d", e.what, e.code) }

// download sends req and returns the picture it answers with. It insists on a 200, on a picture
// type the cache can store, and on a body within MaxImageBytes. what names the picture in errors.
func download(client *http.Client, req *http.Request, what string) (Image, error) {
	resp, err := client.Do(req)
	if err != nil {
		return Image{}, fmt.Errorf("%s: %w", what, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return Image{}, &statusError{what: what, code: resp.StatusCode}
	}
	contentType := resp.Header.Get("Content-Type")
	if _, ok := extensionFor(contentType); !ok {
		return Image{}, fmt.Errorf("%s: content type %q is not a supported picture", what, contentType)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxImageBytes+1))
	if err != nil {
		return Image{}, fmt.Errorf("%s: read body: %w", what, err)
	}
	if len(data) > MaxImageBytes {
		return Image{}, fmt.Errorf("%s: larger than %d bytes", what, MaxImageBytes)
	}
	return Image{ContentType: contentType, Data: data}, nil
}
