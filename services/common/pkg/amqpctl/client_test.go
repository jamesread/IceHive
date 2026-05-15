package amqpctl

import (
	"context"
	"testing"

	controlv1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/control/v1"
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
