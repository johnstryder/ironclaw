package tooling

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	ftypes "github.com/h2non/filetype/types"
)

// =============================================================================
// Test Doubles
// =============================================================================

// mockImageProcessor is a test double for ImageProcessor.
type mockImageProcessor struct {
	detectTypeResult string
	detectTypeErr    error
	dimensionsW      int
	dimensionsH      int
	dimensionsErr    error
	resizeErr        error
	grayscaleErr     error
}

func (m *mockImageProcessor) DetectType(path string) (string, error) {
	return m.detectTypeResult, m.detectTypeErr
}

func (m *mockImageProcessor) GetDimensions(path string) (int, int, error) {
	return m.dimensionsW, m.dimensionsH, m.dimensionsErr
}

func (m *mockImageProcessor) Resize(inputPath, outputPath string, width, height int) error {
	return m.resizeErr
}

func (m *mockImageProcessor) ConvertToGrayscale(inputPath, outputPath string) error {
	return m.grayscaleErr
}

// =============================================================================
// ImageTool — Name, Description, Definition
// =============================================================================

func TestImageTool_Name_ShouldReturnImage(t *testing.T) {
	tool := NewImageTool(&mockImageProcessor{})
	if tool.Name() != "image" {
		t.Errorf("Expected name 'image', got '%s'", tool.Name())
	}
}

func TestImageTool_Description_ShouldReturnMeaningfulDescription(t *testing.T) {
	tool := NewImageTool(&mockImageProcessor{})
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestImageTool_Definition_ShouldContainOperationProperty(t *testing.T) {
	tool := NewImageTool(&mockImageProcessor{})
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	if parsed["type"] != "object" {
		t.Errorf("Expected schema type 'object', got %v", parsed["type"])
	}
	props, ok := parsed["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'properties' in schema")
	}
	if _, exists := props["operation"]; !exists {
		t.Error("Expected 'operation' property in schema")
	}
	if _, exists := props["path"]; !exists {
		t.Error("Expected 'path' property in schema")
	}
}

func TestImageTool_Definition_ShouldRequireOperationAndPath(t *testing.T) {
	tool := NewImageTool(&mockImageProcessor{})
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("Expected 'required' array in schema")
	}
	requiredFields := make(map[string]bool)
	for _, r := range required {
		requiredFields[r.(string)] = true
	}
	if !requiredFields["operation"] {
		t.Error("Expected 'operation' in required fields")
	}
	if !requiredFields["path"] {
		t.Error("Expected 'path' in required fields")
	}
}

// =============================================================================
// ImageTool.Call — Input Validation
// =============================================================================

func TestImageTool_Call_ShouldRejectInvalidJSON(t *testing.T) {
	tool := NewImageTool(&mockImageProcessor{})
	_, err := tool.Call(json.RawMessage(`{bad json`))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestImageTool_Call_ShouldRejectMissingOperationField(t *testing.T) {
	tool := NewImageTool(&mockImageProcessor{})
	_, err := tool.Call(json.RawMessage(`{"path":"image.png"}`))
	if err == nil {
		t.Fatal("Expected error for missing operation field")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestImageTool_Call_ShouldRejectMissingPathField(t *testing.T) {
	tool := NewImageTool(&mockImageProcessor{})
	_, err := tool.Call(json.RawMessage(`{"operation":"detect_type"}`))
	if err == nil {
		t.Fatal("Expected error for missing path field")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestImageTool_Call_ShouldRejectEmptyPathString(t *testing.T) {
	tool := NewImageTool(&mockImageProcessor{})
	_, err := tool.Call(json.RawMessage(`{"operation":"detect_type","path":""}`))
	if err == nil {
		t.Fatal("Expected error for empty path string")
	}
}

func TestImageTool_Call_ShouldRejectInvalidOperation(t *testing.T) {
	tool := NewImageTool(&mockImageProcessor{})
	_, err := tool.Call(json.RawMessage(`{"operation":"rotate","path":"image.png"}`))
	if err == nil {
		t.Fatal("Expected error for invalid operation")
	}
}

// =============================================================================
// ImageTool.Call — DetectType Operation
// =============================================================================

func TestImageTool_Call_DetectType_ShouldReturnMimeTypeForValidImage(t *testing.T) {
	proc := &mockImageProcessor{
		detectTypeResult: "image/png",
		dimensionsW:      800,
		dimensionsH:      600,
	}
	tool := NewImageTool(proc)
	result, err := tool.Call(json.RawMessage(`{"operation":"detect_type","path":"photo.png"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if !strings.Contains(result.Data, "image/png") {
		t.Errorf("Expected 'image/png' in data, got: %s", result.Data)
	}
}

func TestImageTool_Call_DetectType_ShouldReturnMetadataWithMimeType(t *testing.T) {
	proc := &mockImageProcessor{
		detectTypeResult: "image/jpeg",
		dimensionsW:      1024,
		dimensionsH:      768,
	}
	tool := NewImageTool(proc)
	result, err := tool.Call(json.RawMessage(`{"operation":"detect_type","path":"photo.jpg"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Metadata["mime_type"] != "image/jpeg" {
		t.Errorf("Expected metadata mime_type='image/jpeg', got '%s'", result.Metadata["mime_type"])
	}
	if result.Metadata["operation"] != "detect_type" {
		t.Errorf("Expected metadata operation='detect_type', got '%s'", result.Metadata["operation"])
	}
}

func TestImageTool_Call_DetectType_ShouldReturnDimensionsInMetadata(t *testing.T) {
	proc := &mockImageProcessor{
		detectTypeResult: "image/png",
		dimensionsW:      1920,
		dimensionsH:      1080,
	}
	tool := NewImageTool(proc)
	result, err := tool.Call(json.RawMessage(`{"operation":"detect_type","path":"photo.png"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Metadata["width"] != "1920" {
		t.Errorf("Expected metadata width='1920', got '%s'", result.Metadata["width"])
	}
	if result.Metadata["height"] != "1080" {
		t.Errorf("Expected metadata height='1080', got '%s'", result.Metadata["height"])
	}
}

func TestImageTool_Call_DetectType_ShouldReturnErrorWhenDetectionFails(t *testing.T) {
	proc := &mockImageProcessor{
		detectTypeErr: fmt.Errorf("file not found"),
	}
	tool := NewImageTool(proc)
	_, err := tool.Call(json.RawMessage(`{"operation":"detect_type","path":"missing.png"}`))
	if err == nil {
		t.Fatal("Expected error when detection fails")
	}
	if !strings.Contains(err.Error(), "failed to detect file type") {
		t.Errorf("Expected 'failed to detect file type' in error, got: %v", err)
	}
}

func TestImageTool_Call_DetectType_ShouldReturnNonImageTypeGracefully(t *testing.T) {
	proc := &mockImageProcessor{
		detectTypeResult: "application/pdf",
		dimensionsErr:    fmt.Errorf("not an image"),
	}
	tool := NewImageTool(proc)
	result, err := tool.Call(json.RawMessage(`{"operation":"detect_type","path":"document.pdf"}`))
	if err != nil {
		t.Fatalf("Expected no error for non-image, got: %v", err)
	}
	if !strings.Contains(result.Data, "application/pdf") {
		t.Errorf("Expected 'application/pdf' in data, got: %s", result.Data)
	}
	if result.Metadata["is_image"] != "false" {
		t.Errorf("Expected metadata is_image='false', got '%s'", result.Metadata["is_image"])
	}
}

func TestImageTool_Call_DetectType_ShouldIndicateIsImageTrueForImages(t *testing.T) {
	proc := &mockImageProcessor{
		detectTypeResult: "image/png",
		dimensionsW:      100,
		dimensionsH:      100,
	}
	tool := NewImageTool(proc)
	result, err := tool.Call(json.RawMessage(`{"operation":"detect_type","path":"photo.png"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Metadata["is_image"] != "true" {
		t.Errorf("Expected metadata is_image='true', got '%s'", result.Metadata["is_image"])
	}
}

// =============================================================================
// ImageTool.Call — Resize Operation
// =============================================================================

func TestImageTool_Call_Resize_ShouldSucceedWithValidInput(t *testing.T) {
	proc := &mockImageProcessor{}
	tool := NewImageTool(proc)
	result, err := tool.Call(json.RawMessage(`{"operation":"resize","path":"photo.png","output_path":"resized.png","width":200,"height":100}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data == "" {
		t.Error("Expected non-empty success message")
	}
}

func TestImageTool_Call_Resize_ShouldReturnMetadataWithDimensions(t *testing.T) {
	proc := &mockImageProcessor{}
	tool := NewImageTool(proc)
	result, err := tool.Call(json.RawMessage(`{"operation":"resize","path":"photo.png","output_path":"resized.png","width":200,"height":100}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Metadata["operation"] != "resize" {
		t.Errorf("Expected metadata operation='resize', got '%s'", result.Metadata["operation"])
	}
	if result.Metadata["width"] != "200" {
		t.Errorf("Expected metadata width='200', got '%s'", result.Metadata["width"])
	}
	if result.Metadata["height"] != "100" {
		t.Errorf("Expected metadata height='100', got '%s'", result.Metadata["height"])
	}
	if result.Metadata["output_path"] != "resized.png" {
		t.Errorf("Expected metadata output_path='resized.png', got '%s'", result.Metadata["output_path"])
	}
}

func TestImageTool_Call_Resize_ShouldRejectZeroWidth(t *testing.T) {
	proc := &mockImageProcessor{}
	tool := NewImageTool(proc)
	_, err := tool.Call(json.RawMessage(`{"operation":"resize","path":"photo.png","output_path":"resized.png","width":0,"height":100}`))
	if err == nil {
		t.Fatal("Expected error for zero width")
	}
	if !strings.Contains(err.Error(), "width and height must be positive") {
		t.Errorf("Expected 'width and height must be positive' in error, got: %v", err)
	}
}

func TestImageTool_Call_Resize_ShouldRejectNegativeHeight(t *testing.T) {
	proc := &mockImageProcessor{}
	tool := NewImageTool(proc)
	_, err := tool.Call(json.RawMessage(`{"operation":"resize","path":"photo.png","output_path":"resized.png","width":200,"height":-1}`))
	if err == nil {
		t.Fatal("Expected error for negative height")
	}
	if !strings.Contains(err.Error(), "width and height must be positive") {
		t.Errorf("Expected 'width and height must be positive' in error, got: %v", err)
	}
}

func TestImageTool_Call_Resize_ShouldRejectMissingOutputPath(t *testing.T) {
	proc := &mockImageProcessor{}
	tool := NewImageTool(proc)
	_, err := tool.Call(json.RawMessage(`{"operation":"resize","path":"photo.png","width":200,"height":100}`))
	if err == nil {
		t.Fatal("Expected error for missing output_path")
	}
	if !strings.Contains(err.Error(), "output_path is required") {
		t.Errorf("Expected 'output_path is required' in error, got: %v", err)
	}
}

func TestImageTool_Call_Resize_ShouldReturnErrorWhenResizeFails(t *testing.T) {
	proc := &mockImageProcessor{
		resizeErr: fmt.Errorf("unsupported format"),
	}
	tool := NewImageTool(proc)
	_, err := tool.Call(json.RawMessage(`{"operation":"resize","path":"photo.bmp","output_path":"resized.bmp","width":200,"height":100}`))
	if err == nil {
		t.Fatal("Expected error when resize fails")
	}
	if !strings.Contains(err.Error(), "failed to resize image") {
		t.Errorf("Expected 'failed to resize image' in error, got: %v", err)
	}
}

// =============================================================================
// ImageTool.Call — ConvertToGrayscale Operation
// =============================================================================

func TestImageTool_Call_Grayscale_ShouldSucceedWithValidInput(t *testing.T) {
	proc := &mockImageProcessor{}
	tool := NewImageTool(proc)
	result, err := tool.Call(json.RawMessage(`{"operation":"convert_to_grayscale","path":"photo.png","output_path":"gray.png"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Data == "" {
		t.Error("Expected non-empty success message")
	}
}

func TestImageTool_Call_Grayscale_ShouldReturnMetadataWithPaths(t *testing.T) {
	proc := &mockImageProcessor{}
	tool := NewImageTool(proc)
	result, err := tool.Call(json.RawMessage(`{"operation":"convert_to_grayscale","path":"photo.png","output_path":"gray.png"}`))
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if result.Metadata["operation"] != "convert_to_grayscale" {
		t.Errorf("Expected metadata operation='convert_to_grayscale', got '%s'", result.Metadata["operation"])
	}
	if result.Metadata["output_path"] != "gray.png" {
		t.Errorf("Expected metadata output_path='gray.png', got '%s'", result.Metadata["output_path"])
	}
	if result.Metadata["path"] != "photo.png" {
		t.Errorf("Expected metadata path='photo.png', got '%s'", result.Metadata["path"])
	}
}

func TestImageTool_Call_Grayscale_ShouldRejectMissingOutputPath(t *testing.T) {
	proc := &mockImageProcessor{}
	tool := NewImageTool(proc)
	_, err := tool.Call(json.RawMessage(`{"operation":"convert_to_grayscale","path":"photo.png"}`))
	if err == nil {
		t.Fatal("Expected error for missing output_path")
	}
	if !strings.Contains(err.Error(), "output_path is required") {
		t.Errorf("Expected 'output_path is required' in error, got: %v", err)
	}
}

func TestImageTool_Call_Grayscale_ShouldReturnErrorWhenConversionFails(t *testing.T) {
	proc := &mockImageProcessor{
		grayscaleErr: fmt.Errorf("corrupt image data"),
	}
	tool := NewImageTool(proc)
	_, err := tool.Call(json.RawMessage(`{"operation":"convert_to_grayscale","path":"corrupt.png","output_path":"gray.png"}`))
	if err == nil {
		t.Fatal("Expected error when grayscale conversion fails")
	}
	if !strings.Contains(err.Error(), "failed to convert to grayscale") {
		t.Errorf("Expected 'failed to convert to grayscale' in error, got: %v", err)
	}
}

// =============================================================================
// ImageTool.Call — Unmarshal error path (defense-in-depth)
// =============================================================================

func TestImageTool_Call_ShouldReturnErrorWhenUnmarshalFails(t *testing.T) {
	original := imgUnmarshalFunc
	imgUnmarshalFunc = func(data []byte, v interface{}) error {
		return fmt.Errorf("forced unmarshal failure")
	}
	defer func() { imgUnmarshalFunc = original }()

	tool := NewImageTool(&mockImageProcessor{})
	_, err := tool.Call(json.RawMessage(`{"operation":"detect_type","path":"photo.png"}`))
	if err == nil {
		t.Fatal("Expected error from unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

func TestImageTool_Call_ShouldReturnErrorForUnknownOperationDefenseInDepth(t *testing.T) {
	original := imgUnmarshalFunc
	imgUnmarshalFunc = func(data []byte, v interface{}) error {
		input, ok := v.(*ImageToolInput)
		if !ok {
			return fmt.Errorf("unexpected type")
		}
		input.Operation = "unknown_op"
		input.Path = "image.png"
		return nil
	}
	defer func() { imgUnmarshalFunc = original }()

	tool := NewImageTool(&mockImageProcessor{})
	_, err := tool.Call(json.RawMessage(`{"operation":"detect_type","path":"image.png"}`))
	if err == nil {
		t.Fatal("Expected error for unknown operation (defense-in-depth)")
	}
	if !strings.Contains(err.Error(), "unknown operation") {
		t.Errorf("Expected 'unknown operation' in error, got: %v", err)
	}
}

// =============================================================================
// spyImageProcessor — verifies correct arguments are passed
// =============================================================================

type spyImageProcessor struct {
	detectTypePath    string
	getDimPath        string
	resizeInput       string
	resizeOutput      string
	resizeW           int
	resizeH           int
	grayscaleInput    string
	grayscaleOutput   string
}

func (s *spyImageProcessor) DetectType(path string) (string, error) {
	s.detectTypePath = path
	return "image/png", nil
}

func (s *spyImageProcessor) GetDimensions(path string) (int, int, error) {
	s.getDimPath = path
	return 100, 100, nil
}

func (s *spyImageProcessor) Resize(inputPath, outputPath string, width, height int) error {
	s.resizeInput = inputPath
	s.resizeOutput = outputPath
	s.resizeW = width
	s.resizeH = height
	return nil
}

func (s *spyImageProcessor) ConvertToGrayscale(inputPath, outputPath string) error {
	s.grayscaleInput = inputPath
	s.grayscaleOutput = outputPath
	return nil
}

func TestImageTool_Call_DetectType_ShouldPassPathToProcessor(t *testing.T) {
	spy := &spyImageProcessor{}
	tool := NewImageTool(spy)
	_, err := tool.Call(json.RawMessage(`{"operation":"detect_type","path":"my/photo.png"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if spy.detectTypePath != "my/photo.png" {
		t.Errorf("Expected processor to receive 'my/photo.png', got '%s'", spy.detectTypePath)
	}
	if spy.getDimPath != "my/photo.png" {
		t.Errorf("Expected GetDimensions to receive 'my/photo.png', got '%s'", spy.getDimPath)
	}
}

func TestImageTool_Call_Resize_ShouldPassArgsToProcessor(t *testing.T) {
	spy := &spyImageProcessor{}
	tool := NewImageTool(spy)
	_, err := tool.Call(json.RawMessage(`{"operation":"resize","path":"in.png","output_path":"out.png","width":300,"height":200}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if spy.resizeInput != "in.png" {
		t.Errorf("Expected input 'in.png', got '%s'", spy.resizeInput)
	}
	if spy.resizeOutput != "out.png" {
		t.Errorf("Expected output 'out.png', got '%s'", spy.resizeOutput)
	}
	if spy.resizeW != 300 {
		t.Errorf("Expected width 300, got %d", spy.resizeW)
	}
	if spy.resizeH != 200 {
		t.Errorf("Expected height 200, got %d", spy.resizeH)
	}
}

func TestImageTool_Call_Grayscale_ShouldPassArgsToProcessor(t *testing.T) {
	spy := &spyImageProcessor{}
	tool := NewImageTool(spy)
	_, err := tool.Call(json.RawMessage(`{"operation":"convert_to_grayscale","path":"color.jpg","output_path":"gray.jpg"}`))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if spy.grayscaleInput != "color.jpg" {
		t.Errorf("Expected input 'color.jpg', got '%s'", spy.grayscaleInput)
	}
	if spy.grayscaleOutput != "gray.jpg" {
		t.Errorf("Expected output 'gray.jpg', got '%s'", spy.grayscaleOutput)
	}
}

// =============================================================================
// RealImageProcessor — Integration Tests (real files with temp directory)
// =============================================================================

func TestRealImageProcessor_DetectType_ShouldDetectPNG(t *testing.T) {
	dir := t.TempDir()
	imgPath := dir + "/test.png"
	if err := createTestPNG(imgPath, 10, 10); err != nil {
		t.Fatalf("setup: %v", err)
	}

	proc := &RealImageProcessor{}
	mimeType, err := proc.DetectType(imgPath)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if mimeType != "image/png" {
		t.Errorf("Expected 'image/png', got '%s'", mimeType)
	}
}

func TestRealImageProcessor_DetectType_ShouldDetectJPEG(t *testing.T) {
	dir := t.TempDir()
	// Create a PNG then save as JPEG
	imgPath := dir + "/test.jpg"
	if err := createTestPNG(imgPath, 10, 10); err != nil {
		t.Fatalf("setup: %v", err)
	}

	proc := &RealImageProcessor{}
	mimeType, err := proc.DetectType(imgPath)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if mimeType != "image/jpeg" {
		t.Errorf("Expected 'image/jpeg', got '%s'", mimeType)
	}
}

func TestRealImageProcessor_DetectType_ShouldReturnOctetStreamForUnknownFile(t *testing.T) {
	dir := t.TempDir()
	txtPath := dir + "/test.txt"
	if err := os.WriteFile(txtPath, []byte("hello world this is plain text"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	proc := &RealImageProcessor{}
	mimeType, err := proc.DetectType(txtPath)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if mimeType != "application/octet-stream" {
		t.Errorf("Expected 'application/octet-stream', got '%s'", mimeType)
	}
}

func TestRealImageProcessor_DetectType_ShouldReturnErrorForNonExistentFile(t *testing.T) {
	proc := &RealImageProcessor{}
	_, err := proc.DetectType("/nonexistent-file-12345.png")
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
	if !strings.Contains(err.Error(), "cannot open file") {
		t.Errorf("Expected 'cannot open file' in error, got: %v", err)
	}
}

func TestRealImageProcessor_DetectType_ShouldReturnErrorForEmptyFile(t *testing.T) {
	dir := t.TempDir()
	emptyPath := dir + "/empty.bin"
	if err := os.WriteFile(emptyPath, []byte{}, 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	proc := &RealImageProcessor{}
	_, err := proc.DetectType(emptyPath)
	if err != nil {
		// Empty files may cause read errors — that's acceptable
		if !strings.Contains(err.Error(), "cannot read file header") {
			t.Errorf("Expected 'cannot read file header' in error for empty file, got: %v", err)
		}
	}
}

func TestRealImageProcessor_GetDimensions_ShouldReturnCorrectDimensions(t *testing.T) {
	dir := t.TempDir()
	imgPath := dir + "/sized.png"
	if err := createTestPNG(imgPath, 320, 240); err != nil {
		t.Fatalf("setup: %v", err)
	}

	proc := &RealImageProcessor{}
	w, h, err := proc.GetDimensions(imgPath)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if w != 320 {
		t.Errorf("Expected width 320, got %d", w)
	}
	if h != 240 {
		t.Errorf("Expected height 240, got %d", h)
	}
}

func TestRealImageProcessor_GetDimensions_ShouldReturnErrorForNonExistentFile(t *testing.T) {
	proc := &RealImageProcessor{}
	_, _, err := proc.GetDimensions("/nonexistent-file-12345.png")
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
}

func TestRealImageProcessor_GetDimensions_ShouldReturnErrorForNonImageFile(t *testing.T) {
	dir := t.TempDir()
	txtPath := dir + "/test.txt"
	if err := os.WriteFile(txtPath, []byte("not an image"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	proc := &RealImageProcessor{}
	_, _, err := proc.GetDimensions(txtPath)
	if err == nil {
		t.Fatal("Expected error for non-image file")
	}
}

func TestRealImageProcessor_Resize_ShouldCreateResizedImage(t *testing.T) {
	dir := t.TempDir()
	inputPath := dir + "/input.png"
	outputPath := dir + "/resized.png"
	if err := createTestPNG(inputPath, 400, 300); err != nil {
		t.Fatalf("setup: %v", err)
	}

	proc := &RealImageProcessor{}
	err := proc.Resize(inputPath, outputPath, 200, 150)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Verify the output file exists and has correct dimensions
	w, h, err := proc.GetDimensions(outputPath)
	if err != nil {
		t.Fatalf("cannot read resized image: %v", err)
	}
	if w != 200 {
		t.Errorf("Expected resized width 200, got %d", w)
	}
	if h != 150 {
		t.Errorf("Expected resized height 150, got %d", h)
	}
}

func TestRealImageProcessor_Resize_ShouldReturnErrorForNonExistentInput(t *testing.T) {
	dir := t.TempDir()
	proc := &RealImageProcessor{}
	err := proc.Resize("/nonexistent.png", dir+"/out.png", 100, 100)
	if err == nil {
		t.Fatal("Expected error for non-existent input")
	}
}

func TestRealImageProcessor_ConvertToGrayscale_ShouldCreateGrayscaleImage(t *testing.T) {
	dir := t.TempDir()
	inputPath := dir + "/color.png"
	outputPath := dir + "/gray.png"
	if err := createTestPNG(inputPath, 100, 100); err != nil {
		t.Fatalf("setup: %v", err)
	}

	proc := &RealImageProcessor{}
	err := proc.ConvertToGrayscale(inputPath, outputPath)
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}

	// Verify output exists and is still valid
	w, h, err := proc.GetDimensions(outputPath)
	if err != nil {
		t.Fatalf("cannot read grayscale image: %v", err)
	}
	if w != 100 || h != 100 {
		t.Errorf("Expected 100x100, got %dx%d", w, h)
	}
}

func TestRealImageProcessor_ConvertToGrayscale_ShouldReturnErrorForNonExistentInput(t *testing.T) {
	dir := t.TempDir()
	proc := &RealImageProcessor{}
	err := proc.ConvertToGrayscale("/nonexistent.png", dir+"/out.png")
	if err == nil {
		t.Fatal("Expected error for non-existent input")
	}
}

func TestRealImageProcessor_DetectType_ShouldReturnErrorWhenFiletypeMatchFails(t *testing.T) {
	dir := t.TempDir()
	imgPath := dir + "/test.png"
	if err := createTestPNG(imgPath, 10, 10); err != nil {
		t.Fatalf("setup: %v", err)
	}

	original := filetypeMatchFunc
	filetypeMatchFunc = func(buf []byte) (ftypes.Type, error) {
		return ftypes.Type{}, fmt.Errorf("forced match error")
	}
	defer func() { filetypeMatchFunc = original }()

	proc := &RealImageProcessor{}
	_, err := proc.DetectType(imgPath)
	if err == nil {
		t.Fatal("Expected error when filetype.Match fails")
	}
	if !strings.Contains(err.Error(), "filetype match error") {
		t.Errorf("Expected 'filetype match error' in error, got: %v", err)
	}
}

func TestRealImageProcessor_Resize_ShouldReturnErrorWhenSaveFails(t *testing.T) {
	dir := t.TempDir()
	inputPath := dir + "/input.png"
	if err := createTestPNG(inputPath, 50, 50); err != nil {
		t.Fatalf("setup: %v", err)
	}

	proc := &RealImageProcessor{}
	// Output to a non-existent directory — Save will fail
	err := proc.Resize(inputPath, "/nonexistent-dir-12345/out.png", 25, 25)
	if err == nil {
		t.Fatal("Expected error when save fails")
	}
	if !strings.Contains(err.Error(), "cannot save resized image") {
		t.Errorf("Expected 'cannot save resized image' in error, got: %v", err)
	}
}

func TestRealImageProcessor_ConvertToGrayscale_ShouldReturnErrorWhenSaveFails(t *testing.T) {
	dir := t.TempDir()
	inputPath := dir + "/input.png"
	if err := createTestPNG(inputPath, 50, 50); err != nil {
		t.Fatalf("setup: %v", err)
	}

	proc := &RealImageProcessor{}
	// Output to a non-existent directory — Save will fail
	err := proc.ConvertToGrayscale(inputPath, "/nonexistent-dir-12345/out.png")
	if err == nil {
		t.Fatal("Expected error when save fails")
	}
	if !strings.Contains(err.Error(), "cannot save grayscale image") {
		t.Errorf("Expected 'cannot save grayscale image' in error, got: %v", err)
	}
}

func TestRealImageProcessor_Resize_ShouldReturnErrorForInvalidOutputExtension(t *testing.T) {
	dir := t.TempDir()
	inputPath := dir + "/input.png"
	if err := createTestPNG(inputPath, 50, 50); err != nil {
		t.Fatalf("setup: %v", err)
	}

	proc := &RealImageProcessor{}
	// Save to an unsupported extension
	err := proc.Resize(inputPath, dir+"/out.xyz", 25, 25)
	if err == nil {
		t.Fatal("Expected error for unsupported output extension")
	}
}

// =============================================================================
// End-to-end: ImageTool + RealImageProcessor
// =============================================================================

func TestImageTool_E2E_DetectType_ShouldDetectRealPNG(t *testing.T) {
	dir := t.TempDir()
	imgPath := dir + "/real.png"
	if err := createTestPNG(imgPath, 50, 50); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tool := NewImageTool(&RealImageProcessor{})
	args := fmt.Sprintf(`{"operation":"detect_type","path":"%s"}`, imgPath)
	result, err := tool.Call(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["mime_type"] != "image/png" {
		t.Errorf("Expected mime_type='image/png', got '%s'", result.Metadata["mime_type"])
	}
	if result.Metadata["is_image"] != "true" {
		t.Errorf("Expected is_image='true', got '%s'", result.Metadata["is_image"])
	}
	if result.Metadata["width"] != "50" {
		t.Errorf("Expected width='50', got '%s'", result.Metadata["width"])
	}
	if result.Metadata["height"] != "50" {
		t.Errorf("Expected height='50', got '%s'", result.Metadata["height"])
	}
}

func TestImageTool_E2E_DetectType_ShouldRejectTextFile(t *testing.T) {
	dir := t.TempDir()
	txtPath := dir + "/doc.txt"
	if err := os.WriteFile(txtPath, []byte("just some text content here"), 0644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tool := NewImageTool(&RealImageProcessor{})
	args := fmt.Sprintf(`{"operation":"detect_type","path":"%s"}`, txtPath)
	result, err := tool.Call(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if result.Metadata["is_image"] != "false" {
		t.Errorf("Expected is_image='false' for text file, got '%s'", result.Metadata["is_image"])
	}
}

func TestImageTool_E2E_Resize_ShouldResizeRealImage(t *testing.T) {
	dir := t.TempDir()
	inputPath := dir + "/big.png"
	outputPath := dir + "/small.png"
	if err := createTestPNG(inputPath, 500, 400); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tool := NewImageTool(&RealImageProcessor{})
	args := fmt.Sprintf(`{"operation":"resize","path":"%s","output_path":"%s","width":100,"height":80}`, inputPath, outputPath)
	result, err := tool.Call(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(result.Data, "Successfully resized") {
		t.Errorf("Expected success message, got: %s", result.Data)
	}

	// Verify dimensions of the output
	proc := &RealImageProcessor{}
	w, h, err := proc.GetDimensions(outputPath)
	if err != nil {
		t.Fatalf("cannot read output: %v", err)
	}
	if w != 100 || h != 80 {
		t.Errorf("Expected 100x80, got %dx%d", w, h)
	}
}

func TestImageTool_E2E_Grayscale_ShouldConvertRealImage(t *testing.T) {
	dir := t.TempDir()
	inputPath := dir + "/color.png"
	outputPath := dir + "/gray.png"
	if err := createTestPNG(inputPath, 100, 100); err != nil {
		t.Fatalf("setup: %v", err)
	}

	tool := NewImageTool(&RealImageProcessor{})
	args := fmt.Sprintf(`{"operation":"convert_to_grayscale","path":"%s","output_path":"%s"}`, inputPath, outputPath)
	result, err := tool.Call(json.RawMessage(args))
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	if !strings.Contains(result.Data, "Successfully converted") {
		t.Errorf("Expected success message, got: %s", result.Data)
	}
}

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ SchemaTool = (*ImageTool)(nil)
var _ ImageProcessor = (*mockImageProcessor)(nil)
var _ ImageProcessor = (*spyImageProcessor)(nil)
var _ ImageProcessor = (*RealImageProcessor)(nil)
