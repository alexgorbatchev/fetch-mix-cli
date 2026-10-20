package main

import (
	"encoding/json"
	"errors"
)

// A failed download can already have emitted a partial result. Preserve it
// instead of appending a second JSON document when unwinding the pipeline.
type reportedFailure struct{ error }

func (e *reportedFailure) Unwrap() error { return e.error }

func writeJSONResult(value any, runErr error) error {
	err := errors.Join(runErr, json.NewEncoder(resultWriter).Encode(value))
	if err != nil {
		return &reportedFailure{err}
	}
	return nil
}

func writeJSONFailure(err error) error {
	var reported *reportedFailure
	if err == nil || !jsonOutput || errors.As(err, &reported) {
		return err
	}
	return writeJSONResult(struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}{Status: "error", Error: err.Error()}, err)
}
