package amqpctl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	controlv1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/control/v1"
	amqp091 "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
)

const reconnectDelay = 5 * time.Second

// Client publishes and consumes ControlEvent messages on a shared topic exchange.
// Publish uses a single channel guarded by a mutex (amqp091 channels are not thread-safe).
// After a broker or channel drop, publish and consume paths reconnect automatically.
type Client struct {
	conn     *amqp091.Connection
	pubCh    *amqp091.Channel
	cfg      Config
	exchange string
	mu       sync.Mutex
	closed   bool
}

// Connect opens an AMQP connection, declares the control exchange, and prepares a publish channel.
func Connect(_ context.Context, cfg Config) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("amqpctl: empty URL")
	}
	c := &Client{cfg: cfg, exchange: cfg.exchangeName()}
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.connectLocked(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) connectLocked() error {
	connName := strings.TrimSpace(c.cfg.ConnectionName)
	if connName == "" {
		connName = "ih_icehive"
	}
	props := amqp091.NewConnectionProperties()
	props.SetClientConnectionName(connName)
	conn, err := amqp091.DialConfig(c.cfg.URL, amqp091.Config{Properties: props})
	if err != nil {
		return fmt.Errorf("amqp dial: %w", err)
	}
	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("amqp channel: %w", err)
	}
	if err := declareTopicExchange(pubCh, c.exchange); err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return err
	}
	c.conn = conn
	c.pubCh = pubCh
	return nil
}

func declareTopicExchange(ch *amqp091.Channel, name string) error {
	err := ch.ExchangeDeclare(name, amqp091.ExchangeTopic, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declare exchange %q: %w", name, err)
	}
	return nil
}

func (c *Client) closeConnLocked() {
	if c.pubCh != nil {
		_ = c.pubCh.Close()
		c.pubCh = nil
	}
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

//gocyclo:ignore
func (c *Client) ensureConnectedLocked() error {
	if c.closed {
		return fmt.Errorf("amqpctl: client closed")
	}
	if c.conn != nil && !c.conn.IsClosed() {
		if c.pubCh != nil && !c.pubCh.IsClosed() {
			return nil
		}
		return c.resetPubChLocked()
	}
	c.closeConnLocked()
	if err := c.connectLocked(); err != nil {
		return err
	}
	logrus.WithFields(logrus.Fields{
		"url":      c.cfg.URL,
		"exchange": c.exchange,
	}).Info("AMQP status=connected")
	return nil
}

func (c *Client) resetPubChLocked() error {
	if c.pubCh != nil {
		_ = c.pubCh.Close()
		c.pubCh = nil
	}
	pubCh, err := c.conn.Channel()
	if err != nil {
		c.closeConnLocked()
		return fmt.Errorf("amqp channel: %w", err)
	}
	if err := declareTopicExchange(pubCh, c.exchange); err != nil {
		_ = pubCh.Close()
		c.closeConnLocked()
		return err
	}
	c.pubCh = pubCh
	logrus.WithField("exchange", c.exchange).Info("AMQP publish channel reopened")
	return nil
}

func (c *Client) openChannel() (*amqp091.Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.ensureConnectedLocked(); err != nil {
		return nil, err
	}
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("consumer channel: %w", err)
	}
	return ch, nil
}

func isAMQPGone(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, amqp091.ErrClosed) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "channel/connection is not open") ||
		strings.Contains(msg, "Exception (504)")
}

//gocyclo:ignore
func (c *Client) publishLocked(ctx context.Context, routingKey string, pub amqp091.Publishing) error {
	if err := c.ensureConnectedLocked(); err != nil {
		return err
	}
	err := c.pubCh.PublishWithContext(ctx, c.exchange, routingKey, false, false, pub)
	if err == nil {
		return nil
	}
	if !isAMQPGone(err) {
		return err
	}
	logrus.WithError(err).Warn("AMQP status=disconnected; reconnecting")
	c.closeConnLocked()
	if reconnErr := c.connectLocked(); reconnErr != nil {
		return fmt.Errorf("amqp reconnect: %w", reconnErr)
	}
	logrus.WithFields(logrus.Fields{
		"url":      c.cfg.URL,
		"exchange": c.exchange,
	}).Info("AMQP status=connected")
	if err := c.pubCh.PublishWithContext(ctx, c.exchange, routingKey, false, false, pub); err != nil {
		return err
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.publishLocked(ctx, routingKey, pub); err != nil {
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.publishLocked(ctx, routingKey, pub); err != nil {
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
//
//gocyclo:ignore
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
// On connection loss it reconnects and resumes consuming.
func (c *Client) ConsumeControl(ctx context.Context, queueName, bindingKey string, h Handler) error {
	if h == nil {
		return fmt.Errorf("amqpctl: nil Handler")
	}
	return c.consumeLoop(ctx, func() error {
		return c.consumeControlOnce(ctx, queueName, bindingKey, h)
	})
}

//gocyclo:ignore
func (c *Client) consumeControlOnce(ctx context.Context, queueName, bindingKey string, h Handler) error {
	ch, err := c.openChannel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	if bindErr := c.DeclareAndBindQueue(ch, queueName, bindingKey); bindErr != nil {
		return bindErr
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
	return fmt.Errorf("amqp consumer channel closed")
}

// ConsumeJSON binds queueName to the exchange with bindingKey and delivers JSON bodies until ctx is done.
// On connection loss it reconnects and resumes consuming.
func (c *Client) ConsumeJSON(ctx context.Context, queueName, bindingKey string, h JSONHandler) error {
	if h == nil {
		return fmt.Errorf("amqpctl: nil JSONHandler")
	}
	return c.consumeLoop(ctx, func() error {
		return c.consumeJSONOnce(ctx, queueName, bindingKey, h)
	})
}

//gocyclo:ignore
func (c *Client) consumeJSONOnce(ctx context.Context, queueName, bindingKey string, h JSONHandler) error {
	ch, err := c.openChannel()
	if err != nil {
		return err
	}
	defer func() { _ = ch.Close() }()

	if bindErr := c.DeclareAndBindQueue(ch, queueName, bindingKey); bindErr != nil {
		return bindErr
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
	return fmt.Errorf("amqp consumer channel closed")
}

//gocyclo:ignore
func (c *Client) consumeLoop(ctx context.Context, once func() error) error {
	for {
		err := once()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			return nil
		}
		logrus.WithError(err).Warn("AMQP consumer stopped; reconnecting")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(reconnectDelay):
		}
		c.mu.Lock()
		if !c.closed {
			c.closeConnLocked()
			if reconnErr := c.connectLocked(); reconnErr != nil {
				logrus.WithError(reconnErr).Warn("AMQP status=disconnected; reconnect failed")
			} else {
				logrus.WithFields(logrus.Fields{
					"url":      c.cfg.URL,
					"exchange": c.exchange,
				}).Info("AMQP status=connected")
			}
		}
		c.mu.Unlock()
	}
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
	ch, err := c.openChannel()
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
		return nil //nolint:nilerr // malformed messages are dropped after Nack.
	}
	if err := h(ctx, evt); err != nil {
		_ = d.Nack(false, false)
		return nil //nolint:nilerr // handler failures are dropped after Nack.
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
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.closeConnLocked()
	return nil
}
