package tooling

import (
	"encoding/json"
	"fmt"

	invopopSchema "github.com/invopop/jsonschema"
	"github.com/santhosh-tekuri/jsonschema/v5"

	"ironclaw/internal/domain"
)

// CalculatorInput represents the input structure for calculator operations.
type CalculatorInput struct {
	Operation string  `json:"operation" jsonschema:"enum=add,enum=subtract,enum=multiply,enum=divide"`
	A         float64 `json:"a" jsonschema:"minimum=0"`
	B         float64 `json:"b" jsonschema:"minimum=0"`
}

// marshalFunc is the JSON marshaler used by GenerateSchema. Package-level so
// tests can inject a failing marshaler to cover the error return path.
var marshalFunc = func(v interface{}) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// unmarshalFunc is the JSON unmarshaler used by Call. Package-level so tests
// can inject a failing unmarshaler to cover the defense-in-depth error path.
var unmarshalFunc = json.Unmarshal

// GenerateSchema generates a JSON Schema string from a Go struct using
// invopop/jsonschema reflection.
func GenerateSchema(input interface{}) string {
	reflector := invopopSchema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	schema := reflector.Reflect(input)

	schemaBytes, err := marshalFunc(schema)
	if err != nil {
		return ""
	}
	return string(schemaBytes)
}

// ValidateAgainstSchema validates JSON input against a JSON Schema string.
func ValidateAgainstSchema(input json.RawMessage, schemaStr string) error {
	schema, err := jsonschema.CompileString("", schemaStr)
	if err != nil {
		return fmt.Errorf("invalid schema: %w", err)
	}

	var inputData interface{}
	if err := json.Unmarshal(input, &inputData); err != nil {
		return fmt.Errorf("invalid JSON input: %w", err)
	}

	if err := schema.Validate(inputData); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

// calculate performs the arithmetic and is separated from Call so every branch
// (including the default case) can be unit-tested without bypassing schema
// validation.
func calculate(input CalculatorInput) (float64, error) {
	switch input.Operation {
	case "add":
		return input.A + input.B, nil
	case "subtract":
		return input.A - input.B, nil
	case "multiply":
		return input.A * input.B, nil
	case "divide":
		if input.B == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return input.A / input.B, nil
	default:
		return 0, fmt.Errorf("unknown operation: %s", input.Operation)
	}
}

// CalculatorTool is an example implementation of SchemaTool.
type CalculatorTool struct{}

// Name returns the tool name used in function-calling.
func (c *CalculatorTool) Name() string { return "calculator" }

// Description returns a human-readable description for the LLM.
func (c *CalculatorTool) Description() string {
	return "Performs basic arithmetic operations (add, subtract, multiply, divide)"
}

// Definition returns the JSON Schema for calculator input.
func (c *CalculatorTool) Definition() string {
	return GenerateSchema(CalculatorInput{})
}

// Call validates the JSON arguments against the schema and executes the
// calculator operation.
func (c *CalculatorTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	schema := c.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	var input CalculatorInput
	if err := unmarshalFunc(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	result, err := calculate(input)
	if err != nil {
		return nil, err
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("%.2f", result),
		Metadata: map[string]string{
			"operation": input.Operation,
			"a":         fmt.Sprintf("%.2f", input.A),
			"b":         fmt.Sprintf("%.2f", input.B),
		},
	}, nil
}
