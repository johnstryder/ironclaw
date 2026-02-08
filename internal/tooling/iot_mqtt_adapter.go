package tooling

import (
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// PahoMQTTPublisher implements MQTTPublisher using the Eclipse Paho MQTT client.
type PahoMQTTPublisher struct {
	client  mqtt.Client
	qos     byte
	timeout time.Duration
}

// NewPahoMQTTPublisher creates a PahoMQTTPublisher with the given broker URL.
// brokerURL should be in the form "tcp://host:port".
func NewPahoMQTTPublisher(brokerURL, clientID string) *PahoMQTTPublisher {
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(clientID).
		SetAutoReconnect(true).
		SetConnectTimeout(10 * time.Second)

	client := mqtt.NewClient(opts)
	return &PahoMQTTPublisher{
		client:  client,
		qos:     1,
		timeout: 5 * time.Second,
	}
}

// NewPahoMQTTPublisherFromClient creates a PahoMQTTPublisher with a pre-configured client.
// This is useful for testing or advanced configurations.
func NewPahoMQTTPublisherFromClient(client mqtt.Client) *PahoMQTTPublisher {
	return &PahoMQTTPublisher{
		client:  client,
		qos:     1,
		timeout: 5 * time.Second,
	}
}

// Connect establishes the MQTT connection to the broker.
func (p *PahoMQTTPublisher) Connect() error {
	token := p.client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return fmt.Errorf("MQTT connect failed: %w", err)
	}
	return nil
}

// Disconnect gracefully disconnects from the MQTT broker.
func (p *PahoMQTTPublisher) Disconnect() {
	p.client.Disconnect(250)
}

// Publish sends a message to the specified MQTT topic.
func (p *PahoMQTTPublisher) Publish(topic string, payload string) error {
	token := p.client.Publish(topic, p.qos, false, payload)
	if !token.WaitTimeout(p.timeout) {
		return fmt.Errorf("MQTT publish timed out for topic %q", topic)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("MQTT publish failed: %w", err)
	}
	return nil
}

// IsConnected returns true if the client is connected to the broker.
func (p *PahoMQTTPublisher) IsConnected() bool {
	return p.client.IsConnected()
}
