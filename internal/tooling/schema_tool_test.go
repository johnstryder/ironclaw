package tooling

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"ironclaw/internal/domain"
)

// =============================================================================
// Test Doubles
// =============================================================================

// failUnmarshaler always fails json.Unmarshal — used to test the "struct
// unmarshal fails after schema validation passes" path in Call.
type failUnmarshaler struct{}

func (f *failUnmarshaler) UnmarshalJSON([]byte) error {
	return fmt.Errorf("forced unmarshal failure")
}

// inputWithBadField passes schema validation but fails struct unmarshaling.
type inputWithBadField struct {
	Operation string          `json:"operation"`
	A         float64         `json:"a"`
	B         failUnmarshaler `json:"b"`
}

// toolWithBadField is a SchemaTool whose struct has an unmarshaler that fails.
type toolWithBadField struct{}

func (t *toolWithBadField) Name() string        { return "badfieldtool" }
func (t *toolWithBadField) Description() string  { return "tool with bad field" }
func (t *toolWithBadField) Definition() string   { return GenerateSchema(inputWithBadField{}) }
func (t *toolWithBadField) Call(args json.RawMessage) (*domain.ToolResult, error) {
	schema := t.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}
	var input inputWithBadField
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}
	return &domain.ToolResult{Data: "ok"}, nil
}

// looseInput has no enum restriction, so unknown operations reach the default case.
type looseInput struct {
	Operation string  `json:"operation"`
	A         float64 `json:"a" jsonschema:"minimum=0"`
	B         float64 `json:"b" jsonschema:"minimum=0"`
}

// toolWithLooseSchema allows any string for Operation so we can test the
// default switch branch.
type toolWithLooseSchema struct{}

func (t *toolWithLooseSchema) Name() string        { return "loosetool" }
func (t *toolWithLooseSchema) Description() string  { return "tool with loose schema" }
func (t *toolWithLooseSchema) Definition() string   { return GenerateSchema(looseInput{}) }
func (t *toolWithLooseSchema) Call(args json.RawMessage) (*domain.ToolResult, error) {
	schema := t.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}
	var input looseInput
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}
	switch input.Operation {
	case "add":
		return &domain.ToolResult{Data: fmt.Sprintf("%.2f", input.A+input.B)}, nil
	default:
		return nil, fmt.Errorf("unknown operation: %s", input.Operation)
	}
}

// =============================================================================
// GenerateSchema
// =============================================================================

func TestGenerateSchema_ShouldReturnValidJSONSchemaForStruct(t *testing.T) {
	schema := GenerateSchema(CalculatorInput{})
	if schema == "" {
		t.Fatal("Expected non-empty schema")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("Expected type 'object', got %v", parsed["type"])
	}

	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'properties' key")
	}
	for _, key := range []string{"operation", "a", "b"} {
		if _, exists := props[key]; !exists {
			t.Errorf("Expected property '%s'", key)
		}
	}
}

func TestGenerateSchema_ShouldHandleComplexNestedStructs(t *testing.T) {
	type Nested struct {
		Name     string   `json:"name" jsonschema:"minLength=1"`
		Tags     []string `json:"tags,omitempty"`
		Settings struct {
			Enabled bool `json:"enabled"`
		} `json:"settings"`
	}

	schema := GenerateSchema(Nested{})
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}
	props := parsed["properties"].(map[string]interface{})
	for _, key := range []string{"name", "settings"} {
		if _, exists := props[key]; !exists {
			t.Errorf("Expected property '%s'", key)
		}
	}
}

func TestGenerateSchema_ShouldHandleEmptyStruct(t *testing.T) {
	type Empty struct{}
	schema := GenerateSchema(Empty{})
	if schema == "" {
		t.Error("Expected non-empty schema for empty struct")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Errorf("Should be valid JSON: %v", err)
	}
}

func TestGenerateSchema_ShouldReturnEmptyStringWhenMarshalFails(t *testing.T) {
	// Inject a failing marshaler
	original := marshalFunc
	marshalFunc = func(v interface{}) ([]byte, error) {
		return nil, fmt.Errorf("forced marshal error")
	}
	defer func() { marshalFunc = original }()

	schema := GenerateSchema(CalculatorInput{})
	if schema != "" {
		t.Errorf("Expected empty string on marshal failure, got: %q", schema)
	}
}

// =============================================================================
// ValidateAgainstSchema
// =============================================================================

func TestValidateAgainstSchema_ShouldPassForValidInput(t *testing.T) {
	schema := `{
		"type":"object",
		"properties":{"x":{"type":"number"}},
		"required":["x"]
	}`
	err := ValidateAgainstSchema(json.RawMessage(`{"x":42}`), schema)
	if err != nil {
		t.Errorf("Expected pass, got: %v", err)
	}
}

func TestValidateAgainstSchema_ShouldFailForInvalidEnum(t *testing.T) {
	schema := `{
		"type":"object",
		"properties":{"op":{"type":"string","enum":["add","sub"]}},
		"required":["op"]
	}`
	err := ValidateAgainstSchema(json.RawMessage(`{"op":"div"}`), schema)
	if err == nil {
		t.Error("Expected validation failure for invalid enum value")
	}
}

func TestValidateAgainstSchema_ShouldFailForMissingRequired(t *testing.T) {
	schema := `{
		"type":"object",
		"properties":{"x":{"type":"number"}},
		"required":["x"]
	}`
	err := ValidateAgainstSchema(json.RawMessage(`{}`), schema)
	if err == nil {
		t.Error("Expected validation failure for missing required field")
	}
}

func TestValidateAgainstSchema_ShouldReturnInvalidSchemaError(t *testing.T) {
	err := ValidateAgainstSchema(json.RawMessage(`{}`), `{"type":"invalid"}`)
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "invalid schema") {
		t.Errorf("Expected 'invalid schema' in error, got: %v", err)
	}
}

func TestValidateAgainstSchema_ShouldReturnInvalidJSONInputError(t *testing.T) {
	schema := `{"type":"object"}`
	err := ValidateAgainstSchema(json.RawMessage(`{bad`), schema)
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "invalid JSON input") {
		t.Errorf("Expected 'invalid JSON input' in error, got: %v", err)
	}
}

// =============================================================================
// CalculatorTool.Call — operations
// =============================================================================

func TestCalculatorTool_Call_Add(t *testing.T) {
	tool := &CalculatorTool{}
	result, err := tool.Call(json.RawMessage(`{"operation":"add","a":10,"b":5}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Data != "15.00" {
		t.Errorf("want 15.00, got %s", result.Data)
	}
}

func TestCalculatorTool_Call_Subtract(t *testing.T) {
	tool := &CalculatorTool{}
	result, err := tool.Call(json.RawMessage(`{"operation":"subtract","a":10,"b":5}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Data != "5.00" {
		t.Errorf("want 5.00, got %s", result.Data)
	}
}

func TestCalculatorTool_Call_Multiply(t *testing.T) {
	tool := &CalculatorTool{}
	result, err := tool.Call(json.RawMessage(`{"operation":"multiply","a":4,"b":5}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Data != "20.00" {
		t.Errorf("want 20.00, got %s", result.Data)
	}
}

func TestCalculatorTool_Call_Divide(t *testing.T) {
	tool := &CalculatorTool{}
	result, err := tool.Call(json.RawMessage(`{"operation":"divide","a":10,"b":4}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Data != "2.50" {
		t.Errorf("want 2.50, got %s", result.Data)
	}
}

func TestCalculatorTool_Call_ShouldReturnMetadata(t *testing.T) {
	tool := &CalculatorTool{}
	result, err := tool.Call(json.RawMessage(`{"operation":"add","a":1,"b":2}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["operation"] != "add" {
		t.Errorf("want metadata operation=add, got %s", result.Metadata["operation"])
	}
}

func TestCalculatorTool_Call_ShouldRejectDivisionByZero(t *testing.T) {
	tool := &CalculatorTool{}
	_, err := tool.Call(json.RawMessage(`{"operation":"divide","a":10,"b":0}`))
	if err == nil {
		t.Fatal("Expected division by zero error")
	}
	if !strings.Contains(err.Error(), "division by zero") {
		t.Errorf("Expected 'division by zero' in error, got: %v", err)
	}
}

func TestCalculatorTool_Call_ShouldRejectInvalidOperation(t *testing.T) {
	tool := &CalculatorTool{}
	_, err := tool.Call(json.RawMessage(`{"operation":"power","a":2,"b":3}`))
	if err == nil {
		t.Fatal("Expected validation error for invalid operation")
	}
}

func TestCalculatorTool_Call_ShouldRejectWrongTypes(t *testing.T) {
	tool := &CalculatorTool{}
	_, err := tool.Call(json.RawMessage(`{"operation":"add","a":"text","b":3}`))
	if err == nil {
		t.Fatal("Expected validation error for wrong type")
	}
}

func TestCalculatorTool_Call_ShouldRejectMissingField(t *testing.T) {
	tool := &CalculatorTool{}
	_, err := tool.Call(json.RawMessage(`{"operation":"add","a":5}`))
	if err == nil {
		t.Fatal("Expected validation error for missing field")
	}
}

// =============================================================================
// Call — struct unmarshal error path
// =============================================================================

func TestToolCall_ShouldReturnErrorWhenStructUnmarshalFails(t *testing.T) {
	tool := &toolWithBadField{}
	// Pass an empty object for "b" — matches the schema (empty object)
	// but failUnmarshaler.UnmarshalJSON always returns an error.
	args := json.RawMessage(`{"operation":"test","a":1,"b":{}}`)
	_, err := tool.Call(args)
	if err == nil {
		t.Fatal("Expected error from struct unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

// =============================================================================
// Call — unknown operation through loose schema
// =============================================================================

func TestToolCall_ShouldReturnErrorForUnknownOperationWithLooseSchema(t *testing.T) {
	tool := &toolWithLooseSchema{}
	_, err := tool.Call(json.RawMessage(`{"operation":"unknown","a":1,"b":2}`))
	if err == nil {
		t.Fatal("Expected error for unknown operation")
	}
	if !strings.Contains(err.Error(), "unknown operation") {
		t.Errorf("Expected 'unknown operation' in error, got: %v", err)
	}
}

// =============================================================================
// calculate() — direct unit tests for the extracted function
// =============================================================================

func TestCalculate_ShouldReturnErrorForUnknownOperation(t *testing.T) {
	_, err := calculate(CalculatorInput{Operation: "power", A: 2, B: 3})
	if err == nil {
		t.Fatal("Expected error for unknown operation")
	}
	if !strings.Contains(err.Error(), "unknown operation") {
		t.Errorf("Expected 'unknown operation' in error, got: %v", err)
	}
}

func TestCalculate_ShouldAddCorrectly(t *testing.T) {
	v, err := calculate(CalculatorInput{Operation: "add", A: 3, B: 7})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if v != 10 {
		t.Errorf("want 10, got %f", v)
	}
}

func TestCalculate_ShouldSubtractCorrectly(t *testing.T) {
	v, err := calculate(CalculatorInput{Operation: "subtract", A: 10, B: 3})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if v != 7 {
		t.Errorf("want 7, got %f", v)
	}
}

func TestCalculate_ShouldMultiplyCorrectly(t *testing.T) {
	v, err := calculate(CalculatorInput{Operation: "multiply", A: 4, B: 5})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if v != 20 {
		t.Errorf("want 20, got %f", v)
	}
}

func TestCalculate_ShouldDivideCorrectly(t *testing.T) {
	v, err := calculate(CalculatorInput{Operation: "divide", A: 10, B: 4})
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if v != 2.5 {
		t.Errorf("want 2.5, got %f", v)
	}
}

func TestCalculate_ShouldRejectDivisionByZero(t *testing.T) {
	_, err := calculate(CalculatorInput{Operation: "divide", A: 10, B: 0})
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(err.Error(), "division by zero") {
		t.Errorf("Expected 'division by zero', got: %v", err)
	}
}

// =============================================================================
// CalculatorTool.Call — unmarshal error path (defense-in-depth)
// =============================================================================

// calculatorInputBad is identical to CalculatorInput but with a custom
// UnmarshalJSON that always fails. We use it to prove the error path in Call
// works correctly. This is a test-only type.
type calculatorInputBad struct {
	Operation string  `json:"operation" jsonschema:"enum=add"`
	A         float64 `json:"a"`
	B         float64 `json:"b"`
}

func (c *calculatorInputBad) UnmarshalJSON([]byte) error {
	return fmt.Errorf("forced struct unmarshal failure")
}

// badUnmarshalTool proves the unmarshal-error path in a Call implementation
// identical to CalculatorTool but using calculatorInputBad.
type badUnmarshalTool struct{}

func (t *badUnmarshalTool) Name() string        { return "badunmarshal" }
func (t *badUnmarshalTool) Description() string  { return "tool with bad unmarshal" }
func (t *badUnmarshalTool) Definition() string   { return GenerateSchema(calculatorInputBad{}) }
func (t *badUnmarshalTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	schema := t.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}
	var input calculatorInputBad
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}
	return &domain.ToolResult{Data: "ok"}, nil
}

func TestBadUnmarshalTool_Call_ShouldReturnParseError(t *testing.T) {
	tool := &badUnmarshalTool{}
	_, err := tool.Call(json.RawMessage(`{"operation":"add","a":1,"b":2}`))
	if err == nil {
		t.Fatal("Expected error from struct unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

// =============================================================================
// CalculatorTool.Call — unmarshalFunc error path (defense-in-depth)
// =============================================================================

func TestCalculatorTool_Call_ShouldReturnErrorWhenUnmarshalFails(t *testing.T) {
	original := unmarshalFunc
	unmarshalFunc = func(data []byte, v interface{}) error {
		return fmt.Errorf("forced unmarshal failure")
	}
	defer func() { unmarshalFunc = original }()

	tool := &CalculatorTool{}
	_, err := tool.Call(json.RawMessage(`{"operation":"add","a":1,"b":2}`))
	if err == nil {
		t.Fatal("Expected error from unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ SchemaTool = (*CalculatorTool)(nil)
var _ SchemaTool = (*toolWithBadField)(nil)
var _ SchemaTool = (*toolWithLooseSchema)(nil)
var _ SchemaTool = (*badUnmarshalTool)(nil)
