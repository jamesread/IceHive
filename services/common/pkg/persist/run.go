package persist

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/knadh/koanf/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/icehive/icehive/services/common/pkg/amqpctl"
	"github.com/icehive/icehive/services/common/pkg/bootstrap"
	"github.com/icehive/icehive/services/common/pkg/buildinfo"
	"github.com/icehive/icehive/services/common/pkg/common"
	"github.com/icehive/icehive/services/common/pkg/config"
	"github.com/icehive/icehive/services/common/pkg/controllerurl"
	"github.com/icehive/icehive/services/common/pkg/httpshim"
	"github.com/icehive/icehive/services/common/pkg/logging"
	"github.com/icehive/icehive/services/common/pkg/obsmetrics"
)

// MainConfig wires a persister binary: metrics/health sidecar, optional YAML, and domain work.
type MainConfig struct {
	Work          func(ctx context.Context, k *koanf.Koanf, log *logrus.Logger, boot *bootstrap.WorkerRuntime, amqpClient *amqpctl.Client) error
	ID            string
	DefaultListen string
	ConfigYAML    string
	// WorkerKind is sent to Controller.WorkerBootstrap; default is "persister".
	WorkerKind string
	// WorkerID is sent to Controller.WorkerBootstrap; default is ID.
	WorkerID string
}

//gocyclo:ignore
func fetchBootstrapWithRetry(
	ctx context.Context,
	log *logrus.Logger,
	ctrlBase string,
	kind string,
	workerID string,
) (*bootstrap.WorkerRuntime, string, error) {
	for {
		bootCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		boot, effective, err := bootstrap.Fetch(bootCtx, bootstrap.Params{
			BaseURL: ctrlBase,
			Kind:    kind,
			ID:      workerID,
		})
		cancel()
		if err == nil {
			if effective != strings.TrimRight(strings.TrimSpace(ctrlBase), "/") {
				log.WithFields(logrus.Fields{
					"controller":      effective,
					"controller_from": "frontend ingress /api prefix",
				}).Info("controller URL")
			}
			return boot, effective, nil
		}
		log.WithError(err).Warn("controller bootstrap failed; AMQP status=disconnected; retrying in 10s")
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

func waitForAMQPConnect(ctx context.Context, log *logrus.Logger, boot *bootstrap.WorkerRuntime, connectionName string) (*amqpctl.Client, error) {
	for {
		c, err := amqpctl.Connect(ctx, amqpctl.Config{
			URL:            boot.AMQPURL,
			Exchange:       boot.AMQPExchange,
			ConnectionName: connectionName,
		})
		if err == nil {
			log.WithFields(logrus.Fields{
				"url":      boot.AMQPURL,
				"exchange": boot.AMQPExchange,
			}).Info("AMQP status=connected")
			return c, nil
		}
		log.WithError(err).Warn("AMQP status=disconnected; connect failed; retrying in 10s")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

// Main parses flags, loads optional YAML, runs Work until SIGINT/SIGTERM, then shuts down HTTP.
//
//gocyclo:ignore
func Main(cfg MainConfig) {
	if cfg.Work == nil {
		panic("persist.Main: nil Work")
	}
	if cfg.DefaultListen == "" {
		cfg.DefaultListen = ":8082"
	}
	if cfg.ConfigYAML == "" {
		panic("persist.Main: empty ConfigYAML")
	}

	listen := flag.String("listen", config.DefaultWorkerListen(cfg.DefaultListen), "HTTP listen address for metrics and health")
	cfgDir := flag.String("configdir", ".", "directory containing YAML configuration files")
	controllerURL := flag.String("controller", "", "Controller Connect base URL for bootstrap; if empty, ICEHIVE_CONTROLLER_URL is used")
	flag.Parse()

	log := logrus.New()
	logging.ApplyEnvLogLevel(log)
	log.WithFields(logrus.Fields{
		"version": buildinfo.Version,
		"commit":  buildinfo.Commit,
		"date":    buildinfo.Date,
	}).Infof("%s persister-%s starting", common.ProjectName, cfg.ID)

	k, err := config.LoadOptionalYAML(log, *cfgDir, "persister-"+cfg.ID, cfg.ConfigYAML)
	if err != nil {
		log.WithError(err).Fatal("configuration")
	}

	ctrl := controllerurl.Resolve(*controllerURL)
	ctrlBase := ctrl.URL
	log.WithFields(logrus.Fields{
		"controller":      ctrlBase,
		"controller_from": ctrl.Source,
	}).Info("controller URL")
	if controllerurl.SkipVerify() {
		log.Warn("ICEHIVE_CONTROLLER_SKIPVERIFY is set; TLS certificate verification disabled for controller")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var boot *bootstrap.WorkerRuntime
	kind := strings.TrimSpace(cfg.WorkerKind)
	if kind == "" {
		kind = "persister"
	}
	wid := strings.TrimSpace(cfg.WorkerID)
	if wid == "" {
		wid = cfg.ID
	}
	b, ctrlBase, err := fetchBootstrapWithRetry(ctx, log, ctrlBase, kind, wid)
	if err != nil {
		log.WithError(err).Fatal("controller bootstrap")
	}
	amqpClient, err := waitForAMQPConnect(ctx, log, b, "ih_persister_"+cfg.ID)
	if err != nil {
		log.WithError(err).Fatal("amqp connect")
	}
	amqpClient.StartHeartbeatPublisher(ctx, "persister-"+cfg.ID, buildinfo.Version, 10*time.Second)
	defer func() {
		_ = amqpClient.Close()
		log.Info("AMQP status=disconnected")
	}()
	boot = b
	log.WithFields(logrus.Fields{
		"controller": ctrlBase,
		"exchange":   boot.AMQPExchange,
	}).Info("bootstrap from controller")

	listenAddr := *listen
	if k.Exists("listen") {
		listenAddr = k.String("listen")
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           httpshim.Wrap(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("http server failed")
		}
	}()
	log.Infof("persister-%s listening on %s", cfg.ID, listenAddr)

	obsmetrics.Register()

	workErr := cfg.Work(ctx, k, log, boot, amqpClient)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Fatal("graceful shutdown")
	}
	if workErr != nil && !errors.Is(workErr, context.Canceled) {
		log.WithError(workErr).Fatal("persister work")
	}
}
