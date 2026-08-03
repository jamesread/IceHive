package amqpctl

import (
	"context"
	"errors"
	"testing"

	controlv1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/control/v1"
	amqp091 "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestMarshalControlEventPing(t *testing.T) {
	evt := &controlv1.ControlEvent{
		CorrelationId: "corr-1",
		CreatedUnixMs: 42,
		Payload:       &controlv1.ControlEvent_Ping{Ping: &controlv1.Ping{SourceService: "controller"}},
	}
	raw, err := proto.Marshal(evt)
	require.NoError(t, err)

	out := &controlv1.ControlEvent{}
	require.NoError(t, proto.Unmarshal(raw, out))
	assert.Equal(t, evt.GetCorrelationId(), out.GetCorrelationId())
	assert.Equal(t, "controller", out.GetPing().GetSourceService())
}

func TestConnectRejectsEmptyURL(t *testing.T) {
	_, err := Connect(context.Background(), Config{})
	assert.Error(t, err)
}

func TestQueueName(t *testing.T) {
	assert.Equal(t, "ih_foo", QueueName("foo"))
	assert.Equal(t, "ih_foo", QueueName("ih_foo"))
	assert.Equal(t, "ih_", QueueName(""))
	assert.Equal(t, "ih_", QueueName("   "))
}

func TestIsAMQPGone(t *testing.T) {
	assert.False(t, isAMQPGone(nil))
	assert.False(t, isAMQPGone(errors.New("some other error")))
	assert.True(t, isAMQPGone(amqp091.ErrClosed))
	assert.True(t, isAMQPGone(errors.New(`publish: Exception (504) Reason: "channel/connection is not open"`)))
}

func TestPublishAfterClose(t *testing.T) {
	c := &Client{closed: true, cfg: Config{URL: "amqp://example"}, exchange: DefaultControlExchange}
	err := c.PublishJSON(context.Background(), RoutingKeyCollectorEntities, []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "client closed")
}
