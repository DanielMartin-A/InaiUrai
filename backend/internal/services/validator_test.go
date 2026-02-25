package services

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func schemasDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "schemas")
}

func newTestValidator(t *testing.T) *Validator {
	t.Helper()
	v, err := NewValidator(context.Background(), schemasDir(t))
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}
	return v
}

func TestValidateInput_Research_Valid(t *testing.T) {
	v := newTestValidator(t)

	input := json.RawMessage(`{"query":"Latest AI trends","depth":"standard","max_sources":5}`)
	if err := v.ValidateInput(context.Background(), CapabilityResearch, input); err != nil {
		t.Fatalf("expected valid research input, got: %v", err)
	}
}

func TestValidateInput_Research_Invalid(t *testing.T) {
	v := newTestValidator(t)

	cases := []struct {
		name  string
		input string
	}{
		{
			name:  "missing query field",
			input: `{"depth":"standard","max_sources":5}`,
		},
		{
			name:  "query too short (minLength 3)",
			input: `{"query":"ab","depth":"standard","max_sources":5}`,
		},
		{
			name:  "unknown field (additionalProperties: false)",
			input: `{"query":"valid query","depth":"standard","max_sources":5,"extra_field":"boom"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := v.ValidateInput(context.Background(), CapabilityResearch, json.RawMessage(tc.input))
			if err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !errors.Is(err, ErrValidation) {
				t.Errorf("expected ErrValidation, got: %v", err)
			}
		})
	}
}

func TestValidateInput_DataExtraction(t *testing.T) {
	v := newTestValidator(t)

	input := json.RawMessage(`{
		"text": "John Doe, age 30, lives in NYC.",
		"fields": [
			{"name": "full_name", "type": "string", "description": "Person name"},
			{"name": "age", "type": "integer", "description": "Age in years"}
		]
	}`)
	if err := v.ValidateInput(context.Background(), CapabilityDataExtraction, input); err != nil {
		t.Fatalf("expected valid data_extraction input, got: %v", err)
	}
}

func TestValidateOutput_Research_Partial(t *testing.T) {
	v := newTestValidator(t)

	output := json.RawMessage(`{
		"status": "partial",
		"findings": "Some partial findings.",
		"key_points": ["incomplete"],
		"sources": []
	}`)
	if err := v.ValidateOutput(context.Background(), CapabilityResearch, output); err != nil {
		t.Fatalf("expected partial research output to be valid, got: %v", err)
	}
}

func TestGetDeadline(t *testing.T) {
	v := newTestValidator(t)

	tests := []struct {
		name     string
		input    json.RawMessage
		expected time.Duration
	}{
		{name: "deep", input: json.RawMessage(`{"depth":"deep"}`), expected: 45 * time.Second},
		{name: "quick", input: json.RawMessage(`{"depth":"quick"}`), expected: 15 * time.Second},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, err := v.GetDeadline(CapabilityResearch, tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d != tc.expected {
				t.Errorf("got %v, want %v", d, tc.expected)
			}
		})
	}
}

func TestGetCapabilities(t *testing.T) {
	v := newTestValidator(t)

	expected := map[string]time.Duration{
		CapabilityResearch:       30 * time.Second,
		CapabilitySummarize:      15 * time.Second,
		CapabilityDataExtraction: 20 * time.Second,
	}

	if got := len(v.inputSchemas); got != 3 {
		t.Fatalf("expected 3 input schemas, got %d", got)
	}
	if got := len(v.outputSchemas); got != 3 {
		t.Fatalf("expected 3 output schemas, got %d", got)
	}

	for capability, expectedDeadline := range expected {
		t.Run(capability, func(t *testing.T) {
			if _, ok := v.inputSchemas[capability]; !ok {
				t.Fatalf("missing input schema for %q", capability)
			}
			if _, ok := v.outputSchemas[capability]; !ok {
				t.Fatalf("missing output schema for %q", capability)
			}
			d, err := v.GetDeadline(capability, json.RawMessage(`{"depth":"standard"}`))
			if err != nil {
				t.Fatalf("GetDeadline: %v", err)
			}
			if d != expectedDeadline {
				t.Errorf("deadline: got %v, want %v", d, expectedDeadline)
			}
		})
	}
}
