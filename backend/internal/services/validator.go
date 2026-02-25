package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

const (
	CapabilityResearch        = "research"
	CapabilitySummarize       = "summarize"
	CapabilityDataExtraction  = "data_extraction"
)

// Deadline from CAPABILITY SPECS: Research depth-based; Summarize 15s; Data Extraction 20s.
const (
	ResearchDeadlineQuick   = 15 * time.Second
	ResearchDeadlineStandard = 30 * time.Second
	ResearchDeadlineDeep    = 45 * time.Second
	SummarizeDeadline       = 15 * time.Second
	DataExtractionDeadline  = 20 * time.Second
)

type Validator struct {
	inputSchemas  map[string]*jsonschema.Schema
	outputSchemas map[string]*jsonschema.Schema
}

// NewValidator loads all *.json schema files from schemaDir and compiles input_schema and output_schema per capability.
// schemaDir is the path to the schemas directory (e.g. "schemas" or "../schemas" when running from backend/).
func NewValidator(ctx context.Context, schemaDir string) (*Validator, error) {
	_ = ctx
	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		return nil, fmt.Errorf("read schema dir %q: %w", schemaDir, err)
	}
	inputSchemas := make(map[string]*jsonschema.Schema)
	outputSchemas := make(map[string]*jsonschema.Schema)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		capability := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		capability = strings.TrimSuffix(capability, ".v1")
		path := filepath.Join(schemaDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %q: %w", path, err)
		}
		var file struct {
			Properties struct {
				InputSchema  json.RawMessage `json:"input_schema"`
				OutputSchema json.RawMessage `json:"output_schema"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(data, &file); err != nil {
			return nil, fmt.Errorf("parse %q: %w", path, err)
		}
		if len(file.Properties.InputSchema) == 0 || len(file.Properties.OutputSchema) == 0 {
			return nil, fmt.Errorf("%q: missing input_schema or output_schema", path)
		}
		wrapper := file.Properties
		inputID := "https://inaiurai.dev/schemas/" + capability + ".input"
		outputID := "https://inaiurai.dev/schemas/" + capability + ".output"
		inputSchemas[capability], err = jsonschema.CompileString(inputID, string(wrapper.InputSchema))
		if err != nil {
			return nil, fmt.Errorf("compile input schema %q: %w", capability, err)
		}
		outputSchemas[capability], err = jsonschema.CompileString(outputID, string(wrapper.OutputSchema))
		if err != nil {
			return nil, fmt.Errorf("compile output schema %q: %w", capability, err)
		}
	}

	return &Validator{
		inputSchemas:  inputSchemas,
		outputSchemas: outputSchemas,
	}, nil
}

// GetDeadline returns the deadline for the capability based on CAPABILITY SPECS.
// For research, uses input["depth"]: quick=15s, standard=30s, deep=45s.
// For summarize returns 15s; for data_extraction returns 20s.
func (v *Validator) GetDeadline(capability string, input json.RawMessage) (time.Duration, error) {
	switch capability {
	case CapabilityResearch:
		var in struct {
			Depth string `json:"depth"`
		}
		if err := json.Unmarshal(input, &in); err != nil {
			return 0, fmt.Errorf("research input: %w", err)
		}
		switch in.Depth {
		case "quick":
			return ResearchDeadlineQuick, nil
		case "standard":
			return ResearchDeadlineStandard, nil
		case "deep":
			return ResearchDeadlineDeep, nil
		default:
			return ResearchDeadlineStandard, nil
		}
	case CapabilitySummarize:
		return SummarizeDeadline, nil
	case CapabilityDataExtraction:
		return DataExtractionDeadline, nil
	default:
		return 0, fmt.Errorf("unknown capability %q", capability)
	}
}

// ValidateInput performs hard reject: returns an error if input does not match the capability's input_schema.
func (v *Validator) ValidateInput(ctx context.Context, capability string, input json.RawMessage) error {
	schema, ok := v.inputSchemas[capability]
	if !ok {
		return fmt.Errorf("unknown capability %q", capability)
	}
	var doc interface{}
	if err := json.Unmarshal(input, &doc); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return nil
}

// ValidateOutput performs soft flag: returns an error if output does not match the capability's output_schema.
// Callers may treat this as a non-fatal flag (e.g. log and flag task) rather than hard reject.
func (v *Validator) ValidateOutput(ctx context.Context, capability string, output json.RawMessage) error {
	schema, ok := v.outputSchemas[capability]
	if !ok {
		return fmt.Errorf("unknown capability %q", capability)
	}
	var doc interface{}
	if err := json.Unmarshal(output, &doc); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if err := schema.Validate(doc); err != nil {
		return fmt.Errorf("%w: %v", ErrValidation, err)
	}
	return nil
}

// ErrValidation can be used with errors.Is to detect validation failures (hard reject or soft flag).
var ErrValidation = errors.New("validation failed")
