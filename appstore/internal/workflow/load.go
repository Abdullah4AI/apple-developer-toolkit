package workflow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/tidwall/jsonc"
)

var (
	// ErrWorkflowRead indicates workflow file read failure.
	ErrWorkflowRead = errors.New("read workflow")
	// ErrWorkflowParseJSON indicates workflow JSON decode failure.
	ErrWorkflowParseJSON = errors.New("parse workflow JSON")
)

// DefaultPath is the default location for the workflow definition file.
const DefaultPath = ".asc/workflow.json"

// Load reads and validates a workflow definition file.
func Load(path string) (*Definition, error) {
	def, err := LoadUnvalidated(path)
	if err != nil {
		return nil, err
	}
	if errs := Validate(def); len(errs) > 0 {
		joined := make([]error, len(errs))
		for i, e := range errs {
			joined[i] = e
		}
		return nil, errors.Join(joined...)
	}
	return def, nil
}

// LoadUnvalidated reads and parses a workflow definition file without validation.
func LoadUnvalidated(path string) (*Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWorkflowRead, err)
	}

	// Allow JSONC-style comments (// and /* */) in workflow files.
	data = jsonc.ToJSON(data)

	var def Definition
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&def); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrWorkflowParseJSON, err)
	}
	// Ensure there is exactly one JSON value in the file.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("%w: trailing data", ErrWorkflowParseJSON)
	}

	return &def, nil
}
