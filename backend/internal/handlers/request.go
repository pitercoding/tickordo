package handlers

import (
	"encoding/json"
	"errors"
	"io"
)

// errTrailingData reports content after the first JSON value in a body.
var errTrailingData = errors.New("request body must contain a single JSON value")

// decodeJSONBody decodes body into dst, requiring exactly one JSON value
// whose object keys all match fields of dst. Trailing whitespace is allowed.
func decodeJSONBody(body io.Reader, dst any) error {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	// After the first value only whitespace may remain, so the next token
	// must be io.EOF. decoder.More() is not enough: it reports false for
	// some invalid trailing content (e.g. "]"), letting it through.
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errTrailingData
	}

	return nil
}
