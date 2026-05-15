package amqpctl

import "strings"

const (
	// DefaultControlExchange is the durable topic exchange used for all IceHive AMQP traffic.
	DefaultControlExchange = "ex_icehive"

	// QueueNamePrefix is prepended to every broker queue name declared by IceHive binaries.
	QueueNamePrefix = "ih_"

	// ContentTypeProtobuf is set on AMQP publishing so consumers can select a codec.
	ContentTypeProtobuf = "application/x-protobuf"

	// RoutingKeyControlEvents is the default topic routing key for ControlEvent envelopes.
	RoutingKeyControlEvents = "control.events"

	// RoutingKeyHeartbeats is used for per-service heartbeat pings.
	RoutingKeyHeartbeats = "control.heartbeats"

	// RoutingKeyCollectorEntities is used for collector-emitted normalized entities.
	RoutingKeyCollectorEntities = "collector.entities"

	// RoutingKeyCollectorCollectionRequestPrefix is the AMQP topic prefix for on-demand collection.
	// Publish with suffix "." + collector_type (e.g. collector.collection_request.collector-github).
	RoutingKeyCollectorCollectionRequestPrefix = "collector.collection_request"

	// RoutingKeyCollectorSourceSchemaPrefix is the AMQP topic prefix for versioned SourceSchema JSON documents.
	// Publish with suffix "." + collector_type (e.g. collector.source_schema.collector-github).
	RoutingKeyCollectorSourceSchemaPrefix = "collector.source_schema"

	// ContentTypeJSON is set on AMQP publishing for JSON payloads.
	ContentTypeJSON = "application/json"
)

// Config holds connection parameters for Connect.
type Config struct {
	// URL is an AMQP URI, e.g. amqp://guest:guest@localhost:5672/
	URL string

	// Exchange names the topic exchange; when empty, DefaultControlExchange is used.
	Exchange string

	// ConnectionName is sent to the broker as the AMQP connection_name client property (RabbitMQ UI).
	// When empty, Connect uses "ih_icehive".
	ConnectionName string
}

func (c Config) exchangeName() string {
	if c.Exchange != "" {
		return c.Exchange
	}
	return DefaultControlExchange
}

// QueueName returns the broker queue name for a logical queue name (ih_ prefix applied once).
func QueueName(logical string) string {
	logical = strings.TrimSpace(logical)
	if logical == "" {
		return QueueNamePrefix
	}
	if strings.HasPrefix(logical, QueueNamePrefix) {
		return logical
	}
	return QueueNamePrefix + logical
}

// CollectorCollectionRequestRoutingKey returns the topic routing key for collection requests
// targeting a given collection source collector_type (e.g. collector-github).
func CollectorCollectionRequestRoutingKey(collectorType string) string {
	return RoutingKeyCollectorCollectionRequestPrefix + "." + strings.TrimSpace(collectorType)
}

// CollectorSourceSchemaRoutingKey returns the topic routing key for SourceSchema announcements
// for a given collection source collector_type (e.g. collector-github).
func CollectorSourceSchemaRoutingKey(collectorType string) string {
	return RoutingKeyCollectorSourceSchemaPrefix + "." + strings.TrimSpace(collectorType)
}
