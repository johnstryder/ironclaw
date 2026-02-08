package tooling

import (
	"fmt"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// =============================================================================
// Compile-time interface checks
// =============================================================================

var _ MQTTPublisher = (*PahoMQTTPublisher)(nil)
var _ mqtt.Token = (*mockToken)(nil)
var _ mqtt.Client = (*mockPahoClient)(nil)

// =============================================================================
// Mock Paho Token and Client
// =============================================================================

type mockToken struct {
	err         error
	waitTimeout bool // what WaitTimeout returns
}

func (m *mockToken) Wait() bool                       { return true }
func (m *mockToken) WaitTimeout(d time.Duration) bool { return m.waitTimeout }
func (m *mockToken) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}
func (m *mockToken) Error() error { return m.err }

type mockPahoClient struct {
	connected    bool
	connectToken mqtt.Token
	publishToken mqtt.Token
}

func (m *mockPahoClient) IsConnected() bool                                               { return m.connected }
func (m *mockPahoClient) IsConnectionOpen() bool                                          { return m.connected }
func (m *mockPahoClient) Connect() mqtt.Token                                             { return m.connectToken }
func (m *mockPahoClient) Disconnect(quiesce uint)                                         {}
func (m *mockPahoClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	return m.publishToken
}
func (m *mockPahoClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	return &mockToken{waitTimeout: true}
}
func (m *mockPahoClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	return &mockToken{waitTimeout: true}
}
func (m *mockPahoClient) Unsubscribe(topics ...string) mqtt.Token {
	return &mockToken{waitTimeout: true}
}
func (m *mockPahoClient) AddRoute(topic string, callback mqtt.MessageHandler) {}
func (m *mockPahoClient) OptionsReader() mqtt.ClientOptionsReader {
	return mqtt.NewClient(mqtt.NewClientOptions()).OptionsReader()
}

// =============================================================================
// PahoMQTTPublisher — Constructor
// =============================================================================

func TestNewPahoMQTTPublisher_ShouldReturnNonNilPublisher(t *testing.T) {
	pub := NewPahoMQTTPublisher("tcp://localhost:1883", "test-client")
	if pub == nil {
		t.Fatal("Expected non-nil publisher")
	}
}

func TestNewPahoMQTTPublisher_ShouldSetDefaultQoS(t *testing.T) {
	pub := NewPahoMQTTPublisher("tcp://localhost:1883", "test-client")
	if pub.qos != 1 {
		t.Errorf("Expected QoS 1, got %d", pub.qos)
	}
}

func TestNewPahoMQTTPublisher_ShouldSetDefaultTimeout(t *testing.T) {
	pub := NewPahoMQTTPublisher("tcp://localhost:1883", "test-client")
	if pub.timeout != 5*time.Second {
		t.Errorf("Expected 5s timeout, got %v", pub.timeout)
	}
}

func TestNewPahoMQTTPublisherFromClient_ShouldReturnNonNilPublisher(t *testing.T) {
	opts := mqtt.NewClientOptions().AddBroker("tcp://localhost:1883")
	client := mqtt.NewClient(opts)
	pub := NewPahoMQTTPublisherFromClient(client)
	if pub == nil {
		t.Fatal("Expected non-nil publisher")
	}
}

func TestNewPahoMQTTPublisherFromClient_ShouldUseProvidedClient(t *testing.T) {
	opts := mqtt.NewClientOptions().AddBroker("tcp://localhost:1883")
	client := mqtt.NewClient(opts)
	pub := NewPahoMQTTPublisherFromClient(client)
	if pub.client != client {
		t.Error("Expected publisher to use the provided client")
	}
}

// =============================================================================
// PahoMQTTPublisher — IsConnected (without real broker)
// =============================================================================

func TestPahoMQTTPublisher_IsConnected_ShouldReturnFalseWhenNotConnected(t *testing.T) {
	pub := NewPahoMQTTPublisher("tcp://localhost:1883", "test-client")
	if pub.IsConnected() {
		t.Error("Expected IsConnected() to return false when not connected")
	}
}

// =============================================================================
// PahoMQTTPublisher — Connect (error path without real broker)
// =============================================================================

func TestPahoMQTTPublisher_Connect_ShouldReturnErrorWhenBrokerUnreachable(t *testing.T) {
	opts := mqtt.NewClientOptions().
		AddBroker("tcp://127.0.0.1:1").
		SetConnectTimeout(100 * time.Millisecond)
	client := mqtt.NewClient(opts)
	pub := &PahoMQTTPublisher{client: client, qos: 1, timeout: 1 * time.Second}

	err := pub.Connect()
	if err == nil {
		t.Fatal("Expected error when broker is unreachable")
	}
	if !strings.Contains(err.Error(), "MQTT connect failed") {
		t.Errorf("Expected 'MQTT connect failed' in error, got: %v", err)
	}
}

// =============================================================================
// PahoMQTTPublisher — Publish (error path without real broker)
// =============================================================================

func TestPahoMQTTPublisher_Publish_ShouldReturnErrorWhenNotConnected(t *testing.T) {
	pub := NewPahoMQTTPublisher("tcp://127.0.0.1:1", "test-client")
	err := pub.Publish("test/topic", "payload")
	if err == nil {
		t.Fatal("Expected error when publishing while not connected")
	}
}

// =============================================================================
// PahoMQTTPublisher — Connect success path (via mock client)
// =============================================================================

func TestPahoMQTTPublisher_Connect_ShouldSucceedWhenBrokerReachable(t *testing.T) {
	client := &mockPahoClient{
		connected:    true,
		connectToken: &mockToken{err: nil, waitTimeout: true},
	}
	pub := NewPahoMQTTPublisherFromClient(client)
	err := pub.Connect()
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
}

func TestPahoMQTTPublisher_Connect_ShouldReturnErrorFromToken(t *testing.T) {
	client := &mockPahoClient{
		connected:    false,
		connectToken: &mockToken{err: fmt.Errorf("auth failed"), waitTimeout: true},
	}
	pub := NewPahoMQTTPublisherFromClient(client)
	err := pub.Connect()
	if err == nil {
		t.Fatal("Expected error from connect token")
	}
	if !strings.Contains(err.Error(), "MQTT connect failed") {
		t.Errorf("Expected 'MQTT connect failed' in error, got: %v", err)
	}
}

// =============================================================================
// PahoMQTTPublisher — Publish (via mock client)
// =============================================================================

func TestPahoMQTTPublisher_Publish_ShouldSucceedWhenConnected(t *testing.T) {
	client := &mockPahoClient{
		connected:    true,
		publishToken: &mockToken{err: nil, waitTimeout: true},
	}
	pub := NewPahoMQTTPublisherFromClient(client)
	err := pub.Publish("home/light", "ON")
	if err != nil {
		t.Fatalf("Expected success, got: %v", err)
	}
}

func TestPahoMQTTPublisher_Publish_ShouldReturnErrorOnTimeout(t *testing.T) {
	client := &mockPahoClient{
		connected:    true,
		publishToken: &mockToken{err: nil, waitTimeout: false},
	}
	pub := NewPahoMQTTPublisherFromClient(client)
	err := pub.Publish("home/light", "ON")
	if err == nil {
		t.Fatal("Expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("Expected 'timed out' in error, got: %v", err)
	}
}

func TestPahoMQTTPublisher_Publish_ShouldReturnErrorFromToken(t *testing.T) {
	client := &mockPahoClient{
		connected:    true,
		publishToken: &mockToken{err: fmt.Errorf("publish denied"), waitTimeout: true},
	}
	pub := NewPahoMQTTPublisherFromClient(client)
	err := pub.Publish("home/light", "ON")
	if err == nil {
		t.Fatal("Expected publish error")
	}
	if !strings.Contains(err.Error(), "MQTT publish failed") {
		t.Errorf("Expected 'MQTT publish failed' in error, got: %v", err)
	}
}

// =============================================================================
// PahoMQTTPublisher — IsConnected (via mock client)
// =============================================================================

func TestPahoMQTTPublisher_IsConnected_ShouldReturnTrueWhenConnected(t *testing.T) {
	client := &mockPahoClient{connected: true}
	pub := NewPahoMQTTPublisherFromClient(client)
	if !pub.IsConnected() {
		t.Error("Expected IsConnected() to return true")
	}
}

// =============================================================================
// PahoMQTTPublisher — Disconnect (safe to call when not connected)
// =============================================================================

func TestPahoMQTTPublisher_Disconnect_ShouldNotPanicWhenNotConnected(t *testing.T) {
	pub := NewPahoMQTTPublisher("tcp://localhost:1883", "test-client")
	// Should not panic
	pub.Disconnect()
}

func TestPahoMQTTPublisher_Disconnect_ShouldNotPanicWhenConnected(t *testing.T) {
	client := &mockPahoClient{connected: true}
	pub := NewPahoMQTTPublisherFromClient(client)
	// Should not panic
	pub.Disconnect()
}
