package tooling

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"ironclaw/internal/domain"
)

// MQTTPublisher abstracts MQTT publish operations for testability.
type MQTTPublisher interface {
	Publish(topic string, payload string) error
	IsConnected() bool
}

// HTTPDoer abstracts HTTP request execution for testability.
type HTTPDoer interface {
	Do(method, url, body, token string) (statusCode int, responseBody string, err error)
}

// IoTInput represents the input structure for IoT device control.
type IoTInput struct {
	Action  string `json:"action" jsonschema:"enum=mqtt_publish,enum=http_request"`
	Topic   string `json:"topic,omitempty"`
	Payload string `json:"payload,omitempty"`
	URL     string `json:"url,omitempty"`
	Method  string `json:"method,omitempty" jsonschema:"enum=GET,enum=POST,enum=PUT,enum=DELETE"`
	Body    string `json:"body,omitempty"`
	Token   string `json:"token,omitempty"`
}

// iotUnmarshalFunc is the JSON unmarshaler used by Call. Package-level so
// tests can inject a failing unmarshaler to cover the defense-in-depth error path.
var iotUnmarshalFunc = json.Unmarshal

// IoTTool controls smart home IoT devices via MQTT or HTTP (Home Assistant).
type IoTTool struct {
	mqtt MQTTPublisher
	http HTTPDoer
}

// NewIoTTool creates an IoTTool with the given MQTT publisher and HTTP client.
func NewIoTTool(mqtt MQTTPublisher, http HTTPDoer) *IoTTool {
	return &IoTTool{mqtt: mqtt, http: http}
}

// Name returns the tool name used in function-calling.
func (t *IoTTool) Name() string { return "iot" }

// Description returns a human-readable description for the LLM.
func (t *IoTTool) Description() string {
	return "Controls smart home IoT devices via MQTT publish or HTTP requests to Home Assistant"
}

// Definition returns the JSON Schema for IoT input.
func (t *IoTTool) Definition() string {
	return GenerateSchema(IoTInput{})
}

// Call validates the JSON arguments against the schema and executes the IoT action.
func (t *IoTTool) Call(args json.RawMessage) (*domain.ToolResult, error) {
	// 1. Validate input against JSON schema
	schema := t.Definition()
	if err := ValidateAgainstSchema(args, schema); err != nil {
		return nil, fmt.Errorf("input validation failed: %w", err)
	}

	// 2. Unmarshal input
	var input IoTInput
	if err := iotUnmarshalFunc(args, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}

	// 3. Dispatch to appropriate action handler
	switch input.Action {
	case "mqtt_publish":
		return t.executeMQTT(input)
	case "http_request":
		return t.executeHTTP(input)
	default:
		return nil, fmt.Errorf("unknown action: %s", input.Action)
	}
}

// executeMQTT publishes a message to an MQTT topic.
func (t *IoTTool) executeMQTT(input IoTInput) (*domain.ToolResult, error) {
	if t.mqtt == nil {
		return nil, fmt.Errorf("MQTT publisher not configured")
	}
	if input.Topic == "" {
		return nil, fmt.Errorf("topic must not be empty for mqtt_publish")
	}
	if !t.mqtt.IsConnected() {
		return nil, fmt.Errorf("MQTT broker not connected")
	}
	if err := t.mqtt.Publish(input.Topic, input.Payload); err != nil {
		return nil, fmt.Errorf("MQTT publish failed: %w", err)
	}
	return &domain.ToolResult{
		Data: fmt.Sprintf("Successfully published to topic %q", input.Topic),
		Metadata: map[string]string{
			"action":  "mqtt_publish",
			"topic":   input.Topic,
			"payload": input.Payload,
		},
	}, nil
}

// executeHTTP sends an HTTP request to a Home Assistant or IoT endpoint.
func (t *IoTTool) executeHTTP(input IoTInput) (*domain.ToolResult, error) {
	if t.http == nil {
		return nil, fmt.Errorf("HTTP client not configured")
	}
	if input.URL == "" {
		return nil, fmt.Errorf("url must not be empty for http_request")
	}

	// Default to GET if method is empty
	method := input.Method
	if method == "" {
		method = "GET"
	}

	statusCode, responseBody, err := t.http.Do(method, input.URL, input.Body, input.Token)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}

	return &domain.ToolResult{
		Data: responseBody,
		Metadata: map[string]string{
			"action":      "http_request",
			"method":      method,
			"url":         input.URL,
			"status_code": fmt.Sprintf("%d", statusCode),
		},
	}, nil
}

// =============================================================================
// Real Adapters (production implementations)
// =============================================================================

// RealHTTPDoer implements HTTPDoer using net/http.
type RealHTTPDoer struct {
	Client *http.Client
}

// Do sends an HTTP request and returns the status code, response body, and any error.
func (r *RealHTTPDoer) Do(method, url, body, token string) (int, string, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return 0, "", fmt.Errorf("failed to create request: %w", err)
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := r.Client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, "", fmt.Errorf("failed to read response body: %w", err)
	}

	return resp.StatusCode, string(respBody), nil
}
