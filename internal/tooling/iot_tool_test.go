package tooling

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ SchemaTool = (*IoTTool)(nil)

// =============================================================================
// IoTTool — Name, Description, Definition
// =============================================================================

func TestIoTTool_Name_ShouldReturnIoT(t *testing.T) {
	tool := NewIoTTool(nil, nil)
	if tool.Name() != "iot" {
		t.Errorf("Expected name 'iot', got '%s'", tool.Name())
	}
}

func TestIoTTool_Description_ShouldReturnMeaningfulDescription(t *testing.T) {
	tool := NewIoTTool(nil, nil)
	desc := tool.Description()
	if desc == "" {
		t.Error("Expected non-empty description")
	}
}

func TestIoTTool_Definition_ShouldContainActionProperty(t *testing.T) {
	tool := NewIoTTool(nil, nil)
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
	if _, exists := props["action"]; !exists {
		t.Error("Expected 'action' property in schema")
	}
}

func TestIoTTool_Definition_ShouldRequireActionField(t *testing.T) {
	tool := NewIoTTool(nil, nil)
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	required, ok := parsed["required"].([]interface{})
	if !ok {
		t.Fatal("Expected 'required' array in schema")
	}
	found := false
	for _, r := range required {
		if r == "action" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'action' in required fields")
	}
}

func TestIoTTool_Definition_ShouldContainMQTTProperties(t *testing.T) {
	tool := NewIoTTool(nil, nil)
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	props := parsed["properties"].(map[string]interface{})
	for _, key := range []string{"topic", "payload"} {
		if _, exists := props[key]; !exists {
			t.Errorf("Expected property '%s' in schema for MQTT support", key)
		}
	}
}

func TestIoTTool_Definition_ShouldContainHTTPProperties(t *testing.T) {
	tool := NewIoTTool(nil, nil)
	schema := tool.Definition()

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(schema), &parsed); err != nil {
		t.Fatalf("Schema is not valid JSON: %v", err)
	}
	props := parsed["properties"].(map[string]interface{})
	for _, key := range []string{"url", "method", "body", "token"} {
		if _, exists := props[key]; !exists {
			t.Errorf("Expected property '%s' in schema for HTTP support", key)
		}
	}
}

// =============================================================================
// Test Doubles
// =============================================================================

// mockMQTTPublisher is a test double for MQTTPublisher.
type mockMQTTPublisher struct {
	connected    bool
	publishErr   error
	lastTopic    string
	lastPayload  string
	publishCount int
}

func (m *mockMQTTPublisher) Publish(topic string, payload string) error {
	m.lastTopic = topic
	m.lastPayload = payload
	m.publishCount++
	return m.publishErr
}

func (m *mockMQTTPublisher) IsConnected() bool {
	return m.connected
}

// mockHTTPDoer is a test double for HTTPDoer.
type mockHTTPDoer struct {
	statusCode   int
	responseBody string
	err          error
	lastMethod   string
	lastURL      string
	lastBody     string
	lastToken    string
	callCount    int
}

func (m *mockHTTPDoer) Do(method, url, body, token string) (int, string, error) {
	m.lastMethod = method
	m.lastURL = url
	m.lastBody = body
	m.lastToken = token
	m.callCount++
	return m.statusCode, m.responseBody, m.err
}

// spyMQTTPublisher records whether Publish was called.
type spyMQTTPublisher struct {
	called    bool
	connected bool
}

func (s *spyMQTTPublisher) Publish(topic string, payload string) error {
	s.called = true
	return nil
}

func (s *spyMQTTPublisher) IsConnected() bool {
	return s.connected
}

// spyHTTPDoer records whether Do was called.
type spyHTTPDoer struct {
	called bool
}

func (s *spyHTTPDoer) Do(method, url, body, token string) (int, string, error) {
	s.called = true
	return 200, "", nil
}

// =============================================================================
// IoTTool.Call — Input Validation
// =============================================================================

func TestIoTTool_Call_ShouldRejectInvalidJSON(t *testing.T) {
	tool := NewIoTTool(nil, nil)
	_, err := tool.Call(json.RawMessage(`{bad json`))
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestIoTTool_Call_ShouldRejectMissingActionField(t *testing.T) {
	tool := NewIoTTool(nil, nil)
	_, err := tool.Call(json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("Expected error for missing action field")
	}
	if !strings.Contains(err.Error(), "input validation failed") {
		t.Errorf("Expected 'input validation failed' in error, got: %v", err)
	}
}

func TestIoTTool_Call_ShouldRejectWrongTypeForAction(t *testing.T) {
	tool := NewIoTTool(nil, nil)
	_, err := tool.Call(json.RawMessage(`{"action": 123}`))
	if err == nil {
		t.Fatal("Expected error for wrong type in action field")
	}
}

func TestIoTTool_Call_ShouldRejectInvalidActionEnum(t *testing.T) {
	tool := NewIoTTool(nil, nil)
	_, err := tool.Call(json.RawMessage(`{"action": "invalid_action"}`))
	if err == nil {
		t.Fatal("Expected error for invalid action enum value")
	}
}

// =============================================================================
// IoTTool.Call — MQTT Publish Action
// =============================================================================

func TestIoTTool_Call_MQTT_ShouldPublishSuccessfully(t *testing.T) {
	mqtt := &mockMQTTPublisher{connected: true}
	tool := NewIoTTool(mqtt, nil)
	result, err := tool.Call(json.RawMessage(`{"action":"mqtt_publish","topic":"home/light/living","payload":"ON"}`))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if !strings.Contains(result.Data, "published") {
		t.Errorf("Expected 'published' in result data, got: %s", result.Data)
	}
}

func TestIoTTool_Call_MQTT_ShouldPassCorrectTopicAndPayload(t *testing.T) {
	mqtt := &mockMQTTPublisher{connected: true}
	tool := NewIoTTool(mqtt, nil)
	_, err := tool.Call(json.RawMessage(`{"action":"mqtt_publish","topic":"home/light/living","payload":"ON"}`))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if mqtt.lastTopic != "home/light/living" {
		t.Errorf("Expected topic 'home/light/living', got '%s'", mqtt.lastTopic)
	}
	if mqtt.lastPayload != "ON" {
		t.Errorf("Expected payload 'ON', got '%s'", mqtt.lastPayload)
	}
}

func TestIoTTool_Call_MQTT_ShouldReturnErrorWhenNotConnected(t *testing.T) {
	mqtt := &mockMQTTPublisher{connected: false}
	tool := NewIoTTool(mqtt, nil)
	_, err := tool.Call(json.RawMessage(`{"action":"mqtt_publish","topic":"test","payload":"ON"}`))
	if err == nil {
		t.Fatal("Expected error when MQTT not connected")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("Expected 'not connected' in error, got: %v", err)
	}
}

func TestIoTTool_Call_MQTT_ShouldReturnErrorWhenPublishFails(t *testing.T) {
	mqtt := &mockMQTTPublisher{connected: true, publishErr: fmt.Errorf("broker timeout")}
	tool := NewIoTTool(mqtt, nil)
	_, err := tool.Call(json.RawMessage(`{"action":"mqtt_publish","topic":"test","payload":"ON"}`))
	if err == nil {
		t.Fatal("Expected error when publish fails")
	}
	if !strings.Contains(err.Error(), "broker timeout") {
		t.Errorf("Expected 'broker timeout' in error, got: %v", err)
	}
}

func TestIoTTool_Call_MQTT_ShouldReturnErrorWhenPublisherIsNil(t *testing.T) {
	tool := NewIoTTool(nil, nil)
	_, err := tool.Call(json.RawMessage(`{"action":"mqtt_publish","topic":"test","payload":"ON"}`))
	if err == nil {
		t.Fatal("Expected error when MQTT publisher is nil")
	}
	if !strings.Contains(err.Error(), "MQTT publisher not configured") {
		t.Errorf("Expected 'MQTT publisher not configured' in error, got: %v", err)
	}
}

func TestIoTTool_Call_MQTT_ShouldNotPublishWhenNotConnected(t *testing.T) {
	mqtt := &spyMQTTPublisher{connected: false}
	tool := NewIoTTool(mqtt, nil)
	_, _ = tool.Call(json.RawMessage(`{"action":"mqtt_publish","topic":"test","payload":"ON"}`))
	if mqtt.called {
		t.Error("Publish should NOT have been called when not connected")
	}
}

func TestIoTTool_Call_MQTT_ShouldReturnMetadataWithTopicAndAction(t *testing.T) {
	mqtt := &mockMQTTPublisher{connected: true}
	tool := NewIoTTool(mqtt, nil)
	result, err := tool.Call(json.RawMessage(`{"action":"mqtt_publish","topic":"home/light","payload":"OFF"}`))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if result.Metadata["action"] != "mqtt_publish" {
		t.Errorf("Expected metadata action='mqtt_publish', got '%s'", result.Metadata["action"])
	}
	if result.Metadata["topic"] != "home/light" {
		t.Errorf("Expected metadata topic='home/light', got '%s'", result.Metadata["topic"])
	}
}

func TestIoTTool_Call_MQTT_ShouldReturnErrorWhenTopicIsEmpty(t *testing.T) {
	mqtt := &mockMQTTPublisher{connected: true}
	tool := NewIoTTool(mqtt, nil)
	_, err := tool.Call(json.RawMessage(`{"action":"mqtt_publish","topic":"","payload":"ON"}`))
	if err == nil {
		t.Fatal("Expected error for empty topic")
	}
	if !strings.Contains(err.Error(), "topic") {
		t.Errorf("Expected error about topic, got: %v", err)
	}
}

// =============================================================================
// IoTTool.Call — HTTP Request Action
// =============================================================================

func TestIoTTool_Call_HTTP_ShouldSendGetRequest(t *testing.T) {
	httpDoer := &mockHTTPDoer{statusCode: 200, responseBody: `{"state":"on"}`}
	tool := NewIoTTool(nil, httpDoer)
	result, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"http://homeassistant.local:8123/api/states/light.living","method":"GET","token":"abc123"}`))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if !strings.Contains(result.Data, `{"state":"on"}`) {
		t.Errorf("Expected response body in data, got: %s", result.Data)
	}
}

func TestIoTTool_Call_HTTP_ShouldPassCorrectParameters(t *testing.T) {
	httpDoer := &mockHTTPDoer{statusCode: 200, responseBody: "ok"}
	tool := NewIoTTool(nil, httpDoer)
	_, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"http://ha.local/api/services/light/turn_on","method":"POST","body":"{\"entity_id\":\"light.living\"}","token":"secret"}`))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if httpDoer.lastMethod != "POST" {
		t.Errorf("Expected method 'POST', got '%s'", httpDoer.lastMethod)
	}
	if httpDoer.lastURL != "http://ha.local/api/services/light/turn_on" {
		t.Errorf("Expected correct URL, got '%s'", httpDoer.lastURL)
	}
	if httpDoer.lastBody != `{"entity_id":"light.living"}` {
		t.Errorf("Expected correct body, got '%s'", httpDoer.lastBody)
	}
	if httpDoer.lastToken != "secret" {
		t.Errorf("Expected token 'secret', got '%s'", httpDoer.lastToken)
	}
}

func TestIoTTool_Call_HTTP_ShouldReturnErrorWhenHTTPClientIsNil(t *testing.T) {
	tool := NewIoTTool(nil, nil)
	_, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"http://ha.local/api","method":"GET"}`))
	if err == nil {
		t.Fatal("Expected error when HTTP client is nil")
	}
	if !strings.Contains(err.Error(), "HTTP client not configured") {
		t.Errorf("Expected 'HTTP client not configured' in error, got: %v", err)
	}
}

func TestIoTTool_Call_HTTP_ShouldReturnErrorWhenURLIsEmpty(t *testing.T) {
	httpDoer := &mockHTTPDoer{statusCode: 200}
	tool := NewIoTTool(nil, httpDoer)
	_, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"","method":"GET"}`))
	if err == nil {
		t.Fatal("Expected error for empty URL")
	}
	if !strings.Contains(err.Error(), "url") {
		t.Errorf("Expected error about url, got: %v", err)
	}
}

func TestIoTTool_Call_HTTP_ShouldReturnErrorWhenMethodIsEmpty(t *testing.T) {
	httpDoer := &mockHTTPDoer{statusCode: 200}
	tool := NewIoTTool(nil, httpDoer)
	_, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"http://ha.local/api","method":""}`))
	if err == nil {
		t.Fatal("Expected error for empty method")
	}
	if !strings.Contains(err.Error(), "method") {
		t.Errorf("Expected error about method, got: %v", err)
	}
}

func TestIoTTool_Call_HTTP_ShouldReturnErrorWhenRequestFails(t *testing.T) {
	httpDoer := &mockHTTPDoer{err: fmt.Errorf("connection refused")}
	tool := NewIoTTool(nil, httpDoer)
	_, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"http://ha.local/api","method":"GET"}`))
	if err == nil {
		t.Fatal("Expected error when HTTP request fails")
	}
	if !strings.Contains(err.Error(), "connection refused") {
		t.Errorf("Expected 'connection refused' in error, got: %v", err)
	}
}

func TestIoTTool_Call_HTTP_ShouldReturnMetadata(t *testing.T) {
	httpDoer := &mockHTTPDoer{statusCode: 200, responseBody: "ok"}
	tool := NewIoTTool(nil, httpDoer)
	result, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"http://ha.local/api","method":"POST","body":"test"}`))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if result.Metadata["action"] != "http_request" {
		t.Errorf("Expected metadata action='http_request', got '%s'", result.Metadata["action"])
	}
	if result.Metadata["method"] != "POST" {
		t.Errorf("Expected metadata method='POST', got '%s'", result.Metadata["method"])
	}
	if result.Metadata["url"] != "http://ha.local/api" {
		t.Errorf("Expected metadata url='http://ha.local/api', got '%s'", result.Metadata["url"])
	}
	if result.Metadata["status_code"] != "200" {
		t.Errorf("Expected metadata status_code='200', got '%s'", result.Metadata["status_code"])
	}
}

func TestIoTTool_Call_HTTP_ShouldReturnNonOKStatusAsResult(t *testing.T) {
	httpDoer := &mockHTTPDoer{statusCode: 401, responseBody: "Unauthorized"}
	tool := NewIoTTool(nil, httpDoer)
	result, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"http://ha.local/api","method":"GET"}`))
	if err != nil {
		t.Fatalf("Expected success (non-OK is not a Go error), got: %v", err)
	}
	if result.Metadata["status_code"] != "401" {
		t.Errorf("Expected status_code='401', got '%s'", result.Metadata["status_code"])
	}
	if !strings.Contains(result.Data, "Unauthorized") {
		t.Errorf("Expected 'Unauthorized' in data, got: %s", result.Data)
	}
}

func TestIoTTool_Call_HTTP_ShouldNotCallDoerWhenURLIsEmpty(t *testing.T) {
	httpDoer := &spyHTTPDoer{}
	tool := NewIoTTool(nil, httpDoer)
	_, _ = tool.Call(json.RawMessage(`{"action":"http_request","url":"","method":"GET"}`))
	if httpDoer.called {
		t.Error("HTTP Do should NOT have been called with empty URL")
	}
}

func TestIoTTool_Call_HTTP_ShouldDefaultMethodToGET(t *testing.T) {
	httpDoer := &mockHTTPDoer{statusCode: 200, responseBody: "ok"}
	tool := NewIoTTool(nil, httpDoer)
	result, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"http://ha.local/api"}`))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if httpDoer.lastMethod != "GET" {
		t.Errorf("Expected default method 'GET', got '%s'", httpDoer.lastMethod)
	}
	if result.Metadata["method"] != "GET" {
		t.Errorf("Expected metadata method='GET', got '%s'", result.Metadata["method"])
	}
}

// =============================================================================
// IoTTool.Call — Unmarshal error path (defense-in-depth)
// =============================================================================

func TestIoTTool_Call_ShouldReturnErrorWhenUnmarshalFails(t *testing.T) {
	original := iotUnmarshalFunc
	iotUnmarshalFunc = func(data []byte, v interface{}) error {
		return fmt.Errorf("forced unmarshal failure")
	}
	defer func() { iotUnmarshalFunc = original }()

	mqtt := &mockMQTTPublisher{connected: true}
	tool := NewIoTTool(mqtt, nil)
	_, err := tool.Call(json.RawMessage(`{"action":"mqtt_publish","topic":"test","payload":"on"}`))
	if err == nil {
		t.Fatal("Expected error from unmarshal failure")
	}
	if !strings.Contains(err.Error(), "failed to parse input") {
		t.Errorf("Expected 'failed to parse input' in error, got: %v", err)
	}
}

// =============================================================================
// IoTTool.Call — Unknown action (defense-in-depth via injected unmarshal)
// =============================================================================

func TestIoTTool_Call_ShouldReturnErrorForUnknownAction(t *testing.T) {
	original := iotUnmarshalFunc
	iotUnmarshalFunc = func(data []byte, v interface{}) error {
		if p, ok := v.(*IoTInput); ok {
			p.Action = "unknown_action"
		}
		return nil
	}
	defer func() { iotUnmarshalFunc = original }()

	tool := NewIoTTool(nil, nil)
	_, err := tool.Call(json.RawMessage(`{"action":"mqtt_publish","topic":"test","payload":"ON"}`))
	if err == nil {
		t.Fatal("Expected error for unknown action")
	}
	if !strings.Contains(err.Error(), "unknown action") {
		t.Errorf("Expected 'unknown action' in error, got: %v", err)
	}
}

// =============================================================================
// IoTTool.Call — MQTT payload metadata
// =============================================================================

func TestIoTTool_Call_MQTT_ShouldIncludePayloadInMetadata(t *testing.T) {
	mqtt := &mockMQTTPublisher{connected: true}
	tool := NewIoTTool(mqtt, nil)
	result, err := tool.Call(json.RawMessage(`{"action":"mqtt_publish","topic":"home/light","payload":"brightness:50"}`))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if result.Metadata["payload"] != "brightness:50" {
		t.Errorf("Expected payload in metadata, got '%s'", result.Metadata["payload"])
	}
}

// =============================================================================
// IoTTool.Call — HTTP with PUT and DELETE methods
// =============================================================================

func TestIoTTool_Call_HTTP_ShouldSendPutRequest(t *testing.T) {
	httpDoer := &mockHTTPDoer{statusCode: 200, responseBody: "updated"}
	tool := NewIoTTool(nil, httpDoer)
	result, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"http://ha.local/api","method":"PUT","body":"data"}`))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if httpDoer.lastMethod != "PUT" {
		t.Errorf("Expected method 'PUT', got '%s'", httpDoer.lastMethod)
	}
	if result.Data != "updated" {
		t.Errorf("Expected 'updated', got '%s'", result.Data)
	}
}

func TestIoTTool_Call_HTTP_ShouldSendDeleteRequest(t *testing.T) {
	httpDoer := &mockHTTPDoer{statusCode: 204, responseBody: ""}
	tool := NewIoTTool(nil, httpDoer)
	result, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"http://ha.local/api/entity","method":"DELETE"}`))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if httpDoer.lastMethod != "DELETE" {
		t.Errorf("Expected method 'DELETE', got '%s'", httpDoer.lastMethod)
	}
	if result.Metadata["status_code"] != "204" {
		t.Errorf("Expected status_code='204', got '%s'", result.Metadata["status_code"])
	}
}

// =============================================================================
// IoTTool.Call — HTTP with empty body and no token
// =============================================================================

func TestIoTTool_Call_HTTP_ShouldWorkWithoutBodyOrToken(t *testing.T) {
	httpDoer := &mockHTTPDoer{statusCode: 200, responseBody: "ok"}
	tool := NewIoTTool(nil, httpDoer)
	result, err := tool.Call(json.RawMessage(`{"action":"http_request","url":"http://ha.local/api","method":"GET"}`))
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if httpDoer.lastBody != "" {
		t.Errorf("Expected empty body, got '%s'", httpDoer.lastBody)
	}
	if httpDoer.lastToken != "" {
		t.Errorf("Expected empty token, got '%s'", httpDoer.lastToken)
	}
	if result.Data != "ok" {
		t.Errorf("Expected 'ok', got '%s'", result.Data)
	}
}

// =============================================================================
// RealHTTPDoer — Unit Tests
// =============================================================================

func TestRealHTTPDoer_Do_ShouldReturnErrorForInvalidURL(t *testing.T) {
	doer := &RealHTTPDoer{}
	_, _, err := doer.Do("GET", "://bad-url", "", "")
	if err == nil {
		t.Fatal("Expected error for invalid URL")
	}
}

func TestRealHTTPDoer_Do_ShouldReturnStatusCodeAndBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte(`{"state":"on"}`))
	}))
	defer server.Close()

	doer := &RealHTTPDoer{}
	statusCode, body, err := doer.Do("GET", server.URL, "", "")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if statusCode != 200 {
		t.Errorf("Expected status 200, got %d", statusCode)
	}
	if body != `{"state":"on"}` {
		t.Errorf("Expected body '{\"state\":\"on\"}', got '%s'", body)
	}
}

func TestRealHTTPDoer_Do_ShouldSendAuthorizationHeader(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer server.Close()

	doer := &RealHTTPDoer{}
	_, _, err := doer.Do("GET", server.URL, "", "my-token-123")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if receivedAuth != "Bearer my-token-123" {
		t.Errorf("Expected 'Bearer my-token-123', got '%s'", receivedAuth)
	}
}

func TestRealHTTPDoer_Do_ShouldNotSetAuthWhenTokenEmpty(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer server.Close()

	doer := &RealHTTPDoer{}
	_, _, err := doer.Do("GET", server.URL, "", "")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if receivedAuth != "" {
		t.Errorf("Expected empty auth header, got '%s'", receivedAuth)
	}
}

func TestRealHTTPDoer_Do_ShouldSendBodyAndContentType(t *testing.T) {
	var receivedBody string
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		bodyBytes, _ := io.ReadAll(r.Body)
		receivedBody = string(bodyBytes)
		w.WriteHeader(200)
	}))
	defer server.Close()

	doer := &RealHTTPDoer{}
	_, _, err := doer.Do("POST", server.URL, `{"entity_id":"light.living"}`, "")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if receivedContentType != "application/json" {
		t.Errorf("Expected 'application/json', got '%s'", receivedContentType)
	}
	if receivedBody != `{"entity_id":"light.living"}` {
		t.Errorf("Expected body, got '%s'", receivedBody)
	}
}

func TestRealHTTPDoer_Do_ShouldNotSetContentTypeWhenBodyEmpty(t *testing.T) {
	var receivedContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(200)
	}))
	defer server.Close()

	doer := &RealHTTPDoer{}
	_, _, err := doer.Do("GET", server.URL, "", "")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if receivedContentType != "" {
		t.Errorf("Expected empty content-type, got '%s'", receivedContentType)
	}
}

func TestRealHTTPDoer_Do_ShouldUseCustomHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	doer := &RealHTTPDoer{Client: &http.Client{}}
	statusCode, body, err := doer.Do("GET", server.URL, "", "")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
	if statusCode != 200 {
		t.Errorf("Expected 200, got %d", statusCode)
	}
	if body != "ok" {
		t.Errorf("Expected 'ok', got '%s'", body)
	}
}

func TestRealHTTPDoer_Do_ShouldReturnErrorWhenServerUnreachable(t *testing.T) {
	doer := &RealHTTPDoer{}
	_, _, err := doer.Do("GET", "http://127.0.0.1:1/nonexistent", "", "")
	if err == nil {
		t.Fatal("Expected error for unreachable server")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("Expected 'request failed' in error, got: %v", err)
	}
}

func TestRealHTTPDoer_Do_ShouldReturnNon200StatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte("forbidden"))
	}))
	defer server.Close()

	doer := &RealHTTPDoer{}
	statusCode, body, err := doer.Do("GET", server.URL, "", "")
	if err != nil {
		t.Fatalf("Expected success for non-200, got: %v", err)
	}
	if statusCode != 403 {
		t.Errorf("Expected 403, got %d", statusCode)
	}
	if body != "forbidden" {
		t.Errorf("Expected 'forbidden', got '%s'", body)
	}
}

// brokenBodyTransport returns a response whose Body always errors on Read.
type brokenBodyTransport struct {
	statusCode int
}

func (b *brokenBodyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: b.statusCode,
		Body:       io.NopCloser(&errorReader{}),
		Header:     make(http.Header),
	}, nil
}

type errorReader struct{}

func (e *errorReader) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("forced read error")
}

func TestRealHTTPDoer_Do_ShouldReturnErrorWhenBodyReadFails(t *testing.T) {
	doer := &RealHTTPDoer{
		Client: &http.Client{
			Transport: &brokenBodyTransport{statusCode: 200},
		},
	}
	_, _, err := doer.Do("GET", "http://example.com/test", "", "")
	if err == nil {
		t.Fatal("Expected error when body read fails")
	}
	if !strings.Contains(err.Error(), "failed to read response body") {
		t.Errorf("Expected 'failed to read response body' in error, got: %v", err)
	}
}

func TestRealHTTPDoer_Do_ShouldSendCorrectHTTPMethod(t *testing.T) {
	var receivedMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(200)
	}))
	defer server.Close()

	doer := &RealHTTPDoer{}
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		_, _, err := doer.Do(method, server.URL, "", "")
		if err != nil {
			t.Fatalf("Expected success for %s, got: %v", method, err)
		}
		if receivedMethod != method {
			t.Errorf("Expected method '%s', got '%s'", method, receivedMethod)
		}
	}
}
