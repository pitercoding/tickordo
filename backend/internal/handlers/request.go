package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// errTrailingData reports content after the first JSON value in a body.
var errTrailingData = errors.New("request body must contain a single JSON value")

// decodeJSONBody decodes body into dst, requiring exactly one JSON value
// whose object keys all match fields of dst. Trailing whitespace is allowed.
//
// If body is wrapped by http.MaxBytesReader and exceeds its limit, the
// returned error is the *http.MaxBytesError, so callers can tell it apart.
func decodeJSONBody(body io.Reader, dst any) error {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		return err
	}

	// After the first value only whitespace may remain, so the next token
	// must be io.EOF. decoder.More() is not enough: it reports false for
	// some invalid trailing content (e.g. "]"), letting it through.
	_, err := decoder.Token()
	if errors.Is(err, io.EOF) {
		return nil
	}

	// The limit can be crossed while reading what follows the first value
	// (e.g. trailing whitespace), which is an oversized body, not trailing data.
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return err
	}

	return errTrailingData
}

// writeDecodeError responds to a decodeJSONBody error: 413 when the body
// exceeded the http.MaxBytesReader limit, 400 for any other decoding error.
func writeDecodeError(w http.ResponseWriter, err error) {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(w, "request body must be at most 1 MB", http.StatusRequestEntityTooLarge)
		return
	}

	writeError(w, "invalid request body", http.StatusBadRequest)
}
