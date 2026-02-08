package tooling

import (
	"encoding/json"
	"testing"

	"ironclaw/internal/domain"
)

// =============================================================================
// SchemaTool Interface Contract Tests
// =============================================================================

func TestSchemaTool_Interface_ShouldRequireNameMethod(t *testing.T) {
	// The SchemaTool interface should include Name() string
	var tool SchemaTool = &CalculatorTool{}
	name := tool.Name()
	if name == "" {
		t.Error("Expected non-empty name from SchemaTool.Name()")
	}
}

func TestSchemaTool_Interface_ShouldRequireDescriptionMethod(t *testing.T) {
	// The SchemaTool interface should include Description() string
	var tool SchemaTool = &CalculatorTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description from SchemaTool.Description()")
	}
}

func TestSchemaTool_Interface_ShouldRequireDefinitionMethod(t *testing.T) {
	// Definition() should return a valid JSON Schema string
	var tool SchemaTool = &CalculatorTool{}
	schema := tool.Definition()
	if schema == "" {
		t.Error("Expected non-empty JSON schema from SchemaTool.Definition()")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Errorf("Definition() should return valid JSON, got error: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("Expected schema type 'object', got %v", parsed["type"])
	}
}

func TestSchemaTool_Interface_ShouldRequireCallMethod(t *testing.T) {
	// Call() should accept json.RawMessage and return ToolResult or error
	var tool SchemaTool = &CalculatorTool{}
	args := json.RawMessage(`{"operation": "add", "a": 2, "b": 3}`)
	result, err := tool.Call(args)
	if err != nil {
		t.Fatalf("Expected successful Call, got error: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Data != "5.00" {
		t.Errorf("Expected result '5.00', got '%s'", result.Data)
	}
}

func TestSchemaTool_Call_ShouldValidateInputAgainstSchemaBeforeExecution(t *testing.T) {
	var tool SchemaTool = &CalculatorTool{}

	// Invalid args should return validation error, NOT execute the tool
	invalidArgs := json.RawMessage(`{"operation": "invalid_op", "a": 5, "b": 3}`)
	_, err := tool.Call(invalidArgs)
	if err == nil {
		t.Error("Expected validation error for invalid args, but call succeeded")
	}
}

func TestSchemaTool_Call_ShouldReturnClearErrorForMissingRequiredFields(t *testing.T) {
	var tool SchemaTool = &CalculatorTool{}

	missingField := json.RawMessage(`{"operation": "add", "a": 5}`)
	_, err := tool.Call(missingField)
	if err == nil {
		t.Error("Expected error for missing required field 'b'")
	}
}

// =============================================================================
// CalculatorTool Concrete Implementation Tests
// =============================================================================

func TestCalculatorTool_Name_ShouldReturnCalculator(t *testing.T) {
	tool := &CalculatorTool{}
	if tool.Name() != "calculator" {
		t.Errorf("Expected name 'calculator', got '%s'", tool.Name())
	}
}

func TestCalculatorTool_Description_ShouldReturnMeaningfulDescription(t *testing.T) {
	tool := &CalculatorTool{}
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestCalculatorTool_Definition_ShouldContainAllInputProperties(t *testing.T) {
	tool := &CalculatorTool{}
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}

	properties, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'properties' in schema")
	}

	for _, prop := range []string{"operation", "a", "b"} {
		if _, exists := properties[prop]; !exists {
			t.Errorf("Expected property '%s' in schema", prop)
		}
	}
}

func TestCalculatorTool_ShouldSatisfySchemaToolInterface(t *testing.T) {
	// Compile-time check: CalculatorTool must implement SchemaTool
	var _ SchemaTool = (*CalculatorTool)(nil)

	// Also verify it produces a valid domain.ToolResult
	tool := &CalculatorTool{}
	result, err := tool.Call(json.RawMessage(`{"operation": "multiply", "a": 4, "b": 5}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Must return *domain.ToolResult
	var _ *domain.ToolResult = result
	if result.Data != "20.00" {
		t.Errorf("Expected '20.00', got '%s'", result.Data)
	}
}
