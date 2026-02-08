package tooling

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/h2non/filetype"
	ftypes "github.com/h2non/filetype/types"

	"ironclaw/internal/domain"
)

// ImageProcessor abstracts image file type detection and processing for testability.
type ImageProcessor interface {
	DetectType(path string) (mimeType string, err error)
	GetDimensions(path string) (width int, height int, err error)
	Resize(inputPath, outputPath string, width, height int) error
	ConvertToGrayscale(inputPath, outputPath string) error
}

// ImageToolInput represents the input structure for image operations.
type ImageToolInput struct {
	Operation  string `json:"operation" jsonschema:"enum=detect_type,enum=resize,enum=convert_to_grayscale"`
	Path       string `json:"path" jsonschema:"minLength=1"`
	OutputPath string `json:"output_path,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
}

// imgUnmarshalFunc is the JSON unmarshaler used by Call. Package-level so
// tests can inject a failing unmarshaler to cover the defense-in-depth error path.
var imgUnmarshalFunc = json.Unmarshal

// ImageTool provides image file type detection and basic image processing.
type ImageTool struct {
	processor ImageProcessor
}

// NewImageTool creates an ImageTool with the given image processor.
func NewImageTool(processor ImageProcessor) *ImageTool {
	return &ImageTool{processor: processor}
}

// Name returns the tool name used in function-calling.
func (t *ImageTool) Name() string { return "image" }

// Description returns a human-readable description for the LLM.
func (t *ImageTool) Description() string {
	return "Detects image file types (MIME), retrieves dimensions, resizes images, and converts to grayscale"
}

// Definition returns the JSON Schema for image tool input.
func (t *ImageTool) Definition() string {
	return GenerateSchema(ImageToolInput{})
}

// Call validates the JSON arguments against the schema and executes the image operation.
func (t *ImageTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	// 1. Validate input against JSON schema
	schema := t.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// 2. Unmarshal input
	var input ImageToolInput
	if err := imgUnmarshalFunc(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// 3. Dispatch to the appropriate operation
	switch input.Operation {
	case "detect_type":
		return t.detectType(input.Path)
	case "resize":
		return t.resize(input.Path, input.OutputPath, input.Width, input.Height)
	case "convert_to_grayscale":
		return t.convertToGrayscale(input.Path, input.OutputPath)
	default:
		return nil, fmt.Errorf("unknown operation: %s", input.Operation)
	}
}

func (t *ImageTool) detectType(path string) (*domain.ToolResult, error) {
	mimeType, err := t.processor.DetectType(path)
	if err != nil {
		return nil, fmt.Errorf("failed to detect file type: %w", err)
	}

	isImage := strings.HasPrefix(mimeType, "image/")
	metadata := map[string]string{
		"operation": "detect_type",
		"mime_type": mimeType,
		"path":      path,
		"is_image":  fmt.Sprintf("%t", isImage),
	}

	// If it's an image, try to get dimensions
	if isImage {
		w, h, dimErr := t.processor.GetDimensions(path)
		if dimErr == nil {
			metadata["width"] = fmt.Sprintf("%d", w)
			metadata["height"] = fmt.Sprintf("%d", h)
		}
	}

	data := fmt.Sprintf("File type: %s (is_image: %t)", mimeType, isImage)
	if isImage {
		if w, exists := metadata["width"]; exists {
			data += fmt.Sprintf(", dimensions: %sx%s", w, metadata["height"])
		}
	}

	return &domain.ToolResult{
		Data:     data,
		Metadata: metadata,
	}, nil
}

func (t *ImageTool) resize(path, outputPath string, width, height int) (*domain.ToolResult, error) {
	if outputPath == "" {
		return nil, fmt.Errorf("output_path is required for resize operation")
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("width and height must be positive, got width=%d height=%d", width, height)
	}

	if err := t.processor.Resize(path, outputPath, width, height); err != nil {
		return nil, fmt.Errorf("failed to resize image: %w", err)
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Successfully resized %s to %dx%d -> %s", path, width, height, outputPath),
		Metadata: map[string]string{
			"operation":   "resize",
			"path":        path,
			"output_path": outputPath,
			"width":       fmt.Sprintf("%d", width),
			"height":      fmt.Sprintf("%d", height),
		},
	}, nil
}

func (t *ImageTool) convertToGrayscale(path, outputPath string) (*domain.ToolResult, error) {
	if outputPath == "" {
		return nil, fmt.Errorf("output_path is required for convert_to_grayscale operation")
	}

	if err := t.processor.ConvertToGrayscale(path, outputPath); err != nil {
		return nil, fmt.Errorf("failed to convert to grayscale: %w", err)
	}

	return &domain.ToolResult{
		Data: fmt.Sprintf("Successfully converted %s to grayscale -> %s", path, outputPath),
		Metadata: map[string]string{
			"operation":   "convert_to_grayscale",
			"path":        path,
			"output_path": outputPath,
		},
	}, nil
}

// =============================================================================
// RealImageProcessor — production implementation using filetype + imaging
// =============================================================================

// filetypeMatchFunc is the filetype matcher used by DetectType. Package-level so
// tests can inject a failing matcher to cover the error return path.
var filetypeMatchFunc func([]byte) (ftypes.Type, error) = filetype.Match

// RealImageProcessor implements ImageProcessor using h2non/filetype for MIME
// detection and disintegration/imaging for image manipulation.
type RealImageProcessor struct{}

// DetectType reads the first 261 bytes of the file and uses h2non/filetype
// to determine the MIME type. Returns an error if the file cannot be read
// or the type is unknown.
func (r *RealImageProcessor) DetectType(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot open file: %w", err)
	}
	defer f.Close()

	// filetype only needs the first 261 bytes
	head := make([]byte, 261)
	n, err := f.Read(head)
	if err != nil {
		return "", fmt.Errorf("cannot read file header: %w", err)
	}
	head = head[:n]

	kind, err := filetypeMatchFunc(head)
	if err != nil {
		return "", fmt.Errorf("filetype match error: %w", err)
	}
	if kind == filetype.Unknown {
		return "application/octet-stream", nil
	}

	return kind.MIME.Value, nil
}

// GetDimensions opens the image and returns its width and height.
func (r *RealImageProcessor) GetDimensions(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot open image: %w", err)
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot decode image config: %w", err)
	}

	return cfg.Width, cfg.Height, nil
}

// Resize loads the image, resizes it to the given dimensions, and saves it.
func (r *RealImageProcessor) Resize(inputPath, outputPath string, width, height int) error {
	src, err := imaging.Open(inputPath)
	if err != nil {
		return fmt.Errorf("cannot open image for resize: %w", err)
	}

	dst := imaging.Resize(src, width, height, imaging.Lanczos)

	if err := imaging.Save(dst, outputPath); err != nil {
		return fmt.Errorf("cannot save resized image: %w", err)
	}

	return nil
}

// ConvertToGrayscale loads the image, converts it to grayscale, and saves it.
func (r *RealImageProcessor) ConvertToGrayscale(inputPath, outputPath string) error {
	src, err := imaging.Open(inputPath)
	if err != nil {
		return fmt.Errorf("cannot open image for grayscale: %w", err)
	}

	dst := imaging.Grayscale(src)

	if err := imaging.Save(dst, outputPath); err != nil {
		return fmt.Errorf("cannot save grayscale image: %w", err)
	}

	return nil
}

// createTestPNG creates a minimal valid PNG image at the given path.
// Exported for use in integration tests.
func createTestPNG(path string, width, height int) error {
	img := imaging.New(width, height, color.NRGBA{R: 255, G: 0, B: 0, A: 255})
	return imaging.Save(img, path)
}
