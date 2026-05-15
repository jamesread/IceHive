package main

import (
	"context"

	"github.com/icehive/icehive/services/common/pkg/amqpctl"
	"github.com/icehive/icehive/services/common/pkg/bootstrap"
	"github.com/icehive/icehive/services/common/pkg/persist"
	"github.com/knadh/koanf/v2"
	"github.com/sirupsen/logrus"
)

func main() {
	persist.Main(persist.MainConfig{
		ID:            "yaml",
		DefaultListen: ":8083",
		ConfigYAML:    "persister-yaml.yaml",
		Work:          yamlWork,
	})
}

func yamlWork(ctx context.Context, _ *koanf.Koanf, log *logrus.Logger, boot *bootstrap.WorkerRuntime, _ *amqpctl.Client) error {
	if boot != nil {
		log.WithField("exchange", boot.AMQPExchange).Info("YAML persister ready (AMQP from controller; consumer and store to be wired)")
	} else {
		log.Info("YAML persister ready (set -controller or ICEHIVE_CONTROLLER_URL for AMQP bootstrap)")
	}
	<-ctx.Done()
	return nil
}
