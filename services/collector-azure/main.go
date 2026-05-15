package main

import (
	"context"

	"github.com/icehive/icehive/services/common/pkg/amqpctl"
	"github.com/icehive/icehive/services/common/pkg/bootstrap"
	"github.com/icehive/icehive/services/common/pkg/collector"
	"github.com/icehive/icehive/services/common/pkg/sourceschema"
	"github.com/knadh/koanf/v2"
	"github.com/sirupsen/logrus"
)

func main() {
	collector.Main(collector.MainConfig{
		ID:            "azure",
		DefaultListen: ":8082",
		ConfigYAML:    "collector-azure.yaml",
		Work:          azureWork,
	})
}

func azureWork(ctx context.Context, _ *koanf.Koanf, log *logrus.Logger, boot *bootstrap.WorkerRuntime, amqpClient *amqpctl.Client, _ string) error {
	if boot != nil {
		log.WithField("exchange", boot.AMQPExchange).Info("Azure collector ready (AMQP from controller; schedule and API client to be wired)")
	} else {
		log.Info("Azure collector ready (set -controller or ICEHIVE_CONTROLLER_URL for AMQP bootstrap)")
	}
	if amqpClient != nil {
		if err := sourceschema.Publish(ctx, amqpClient, sourceschema.AzureV1()); err != nil {
			log.WithError(err).Warn("publish SourceSchema failed")
		}
	}
	<-ctx.Done()
	return nil
}
