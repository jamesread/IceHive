// Package amqpctl provides a small AMQP client for publishing and consuming
// protobuf-encoded control messages defined in protocol/icehive/control/v1/control.proto.
//
// The Client serializes publishes on one channel and reconnects after broker or
// channel loss so long-lived collectors keep heartbeats and entity publishes working
// without relying on process restart.
package amqpctl
