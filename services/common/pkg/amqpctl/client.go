package amqpctl

import (
	"context"
	"fmt"
	"strings"
	"time"

	controlv1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/control/v1"
	amqp091 "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

// Client publishes and consumes ControlEvent messages on a shared topic exchange.
type Client struct {
	conn     *amqp091.Connection
	exchange string
	pubCh    *amqp091.Channel
}

// Connect opens an AMQP connection, declares the control exchange, and prepares a publish channel.
func Connect(_ context.Context, cfg Config) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("amqpctl: empty URL")
	}
	connName := strings.TrimSpace(cfg.ConnectionName)
	if connName == "" {
		connName = "ih_icehive"
	}
	props := amqp091.NewConnectionProperties()
	props.SetClientConnectionName(connName)
	conn, err := amqp091.DialConfig(cfg.URL, amqp091.Config{Properties: props})
	if err != nil {
		return nil, fmt.Errorf("amqp dial: %w", err)
	}
	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("amqp channel: %w", err)
	}
	ex := cfg.exchangeName()
	if err := declareTopicExchange(pubCh, ex); err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return nil, err
	}
	return &Client{conn: conn, exchange: ex, pubCh: pubCh}, nil
}

func declareTopicExchange(ch *amqp091.Channel, name string) error {
	err := ch.ExchangeDeclare(name, amqp091.ExchangeTopic, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declare exchange %q: %w", name, err)
	}
	return nil
}

// PublishControl marshals evt and publishes it to the configured exchange.
func (c *Client) PublishControl(ctx context.Context, routingKey string, evt *controlv1.ControlEvent) error {
	if evt == nil {
		return fmt.Errorf("amqpctl: nil ControlEvent")
	}
	body, err := proto.Marshal(evt)
	if err != nil {
		return fmt.Errorf("marshal ControlEvent: %w", err)
	}
	pub := amqp091.Publishing{
		ContentType:  ContentTypeProtobuf,
		Body:         body,
		DeliveryMode: amqp091.Persistent,
	}
	err = c.pubCh.PublishWithContext(ctx, c.exchange, routingKey, false, false, pub)
	if err != nil {
		return fmt.Errorf("publish: %w", err)
	}
	return nil
}

// PublishJSON publishes a JSON payload to the configured exchange/routing key.
func (c *Client) PublishJSON(ctx context.Context, routingKey string, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("amqpctl: empty JSON body")
	}
	pub := amqp091.Publishing{
		ContentType:  ContentTypeJSON,
		Body:         body,
		DeliveryMode: amqp091.Persistent,
	}
	err := c.pubCh.PublishWithContext(ctx, c.exchange, routingKey, false, false, pub)
	if err != nil {
		return fmt.Errorf("publish json: %w", err)
	}
	return nil
}

// PublishHeartbeat sends a Ping control event on the heartbeats routing key.
func (c *Client) PublishHeartbeat(ctx context.Context, serviceName string) error {
	evt := &controlv1.ControlEvent{
		CorrelationId: fmt.Sprintf("hb-%s-%d", serviceName, time.Now().UnixNano()),
		CreatedUnixMs: time.Now().UnixMilli(),
		Payload: &controlv1.ControlEvent_Ping{
			Ping: &controlv1.Ping{SourceService: serviceName},
		},
	}
	return c.PublishControl(ctx, RoutingKeyHeartbeats, evt)
}

// StartHeartbeatPublisher publishes service heartbeat pings on a fixed interval until ctx ends.
func (c *Client) StartHeartbeatPublisher(ctx context.Context, serviceName string, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	go func() {
		if err := c.PublishHeartbeat(ctx, serviceName); err != nil {
			logrus.WithError(err).WithField("service", serviceName).Warn("AMQP heartbeat send failed")
		} else {
			logrus.WithField("service", serviceName).Info("AMQP heartbeat sent")
		}
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := c.PublishHeartbeat(ctx, serviceName); err != nil {
					logrus.WithError(err).WithField("service", serviceName).Warn("AMQP heartbeat send failed")
				} else {
					logrus.WithField("service", serviceName).Info("AMQP heartbeat sent")
				}
			}
		}
	}()
}

// Handler processes a decoded control event.
type Handler func(ctx context.Context, evt *controlv1.ControlEvent) error

// JSONHandler processes a raw JSON AMQP delivery body.
type JSONHandler func(ctx context.Context, body []byte) error

// ConsumeControl binds queueName to the exchange with bindingKey and delivers decoded events to h until ctx is done.
func (c *Client) ConsumeControl(ctx context.Context, queueName, bindingKey string, h Handler) error {
	if h == nil {
		return fmt.Errorf("amqpctl: nil Handler")
	}
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("consumer channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := c.DeclareAndBindQueue(ch, queueName, bindingKey); err != nil {
		return err
	}
	deliveries, err := ch.ConsumeWithContext(ctx, queueName, "icehive-control", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}

	for d := range deliveries {
		if err := deliver(ctx, d, h); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// ConsumeJSON binds queueName to the exchange with bindingKey and delivers JSON bodies until ctx is done.
func (c *Client) ConsumeJSON(ctx context.Context, queueName, bindingKey string, h JSONHandler) error {
	if h == nil {
		return fmt.Errorf("amqpctl: nil JSONHandler")
	}
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("consumer channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	if err := c.DeclareAndBindQueue(ch, queueName, bindingKey); err != nil {
		return err
	}
	deliveries, err := ch.ConsumeWithContext(ctx, queueName, "icehive-json", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume: %w", err)
	}

	for d := range deliveries {
		if len(d.Body) == 0 {
			_ = d.Nack(false, false)
			continue
		}
		if err := h(ctx, d.Body); err != nil {
			_ = d.Nack(false, false)
			continue
		}
		if err := d.Ack(false); err != nil {
			return fmt.Errorf("ack: %w", err)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// DeclareAndBindQueue ensures queueName exists and is bound to bindingKey on the configured exchange.
func (c *Client) DeclareAndBindQueue(ch *amqp091.Channel, queueName, bindingKey string) error {
	if _, err := ch.QueueDeclare(queueName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	if err := ch.QueueBind(queueName, bindingKey, c.exchange, false, nil); err != nil {
		return fmt.Errorf("bind queue: %w", err)
	}
	return nil
}

// EnsureQueue opens a channel and declares/binds queueName to bindingKey.
func (c *Client) EnsureQueue(queueName, bindingKey string) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("queue setup channel: %w", err)
	}
	defer func() { _ = ch.Close() }()
	return c.DeclareAndBindQueue(ch, queueName, bindingKey)
}

func deliver(ctx context.Context, d amqp091.Delivery, h Handler) error {
	evt := &controlv1.ControlEvent{}
	if err := proto.Unmarshal(d.Body, evt); err != nil {
		_ = d.Nack(false, false)
		return nil
	}
	if err := h(ctx, evt); err != nil {
		_ = d.Nack(false, false)
		return nil
	}
	if err := d.Ack(false); err != nil {
		return fmt.Errorf("ack: %w", err)
	}
	return nil
}

// Close closes publish resources and the underlying connection.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var first error
	if c.pubCh != nil {
		if err := c.pubCh.Close(); err != nil && first == nil {
			first = err
		}
	}
	if c.conn != nil {
		if err := c.conn.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
