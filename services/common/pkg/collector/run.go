package collector

import (
	"context"
	"flag"
	"net/http"
	"os"
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
	"github.com/icehive/icehive/services/common/pkg/logging"
	"github.com/icehive/icehive/services/common/pkg/httpshim"
)

// MainConfig wires a collector binary: metrics/health sidecar, optional YAML, and domain work.
type MainConfig struct {
	ID            string
	DefaultListen string
	ConfigYAML    string
	// WorkerKind is sent to Controller.WorkerBootstrap; default is "collector".
	WorkerKind string
	// WorkerID is sent to Controller.WorkerBootstrap; default is ID.
	WorkerID string
	// ControllerBaseURL is the Connect base URL used for bootstrap (same as -controller / ICEHIVE_CONTROLLER_URL).
	Work func(ctx context.Context, k *koanf.Koanf, log *logrus.Logger, boot *bootstrap.WorkerRuntime, amqpClient *amqpctl.Client, controllerBaseURL string) error
}

// DefaultPollIntervalSeconds is how often collectors reload collection sources from the controller when YAML does not set poll_interval_seconds.
const DefaultPollIntervalSeconds = 60

func fetchBootstrapWithRetry(
	ctx context.Context,
	log *logrus.Logger,
	ctrlBase string,
	kind string,
	workerID string,
) (*bootstrap.WorkerRuntime, error) {
	for {
		bootCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		boot, err := bootstrap.Fetch(bootCtx, bootstrap.Params{
			BaseURL:    ctrlBase,
			HTTPClient: http.DefaultClient,
			Kind:       kind,
			ID:         workerID,
		})
		cancel()
		if err == nil {
			return boot, nil
		}
		log.WithError(err).Warn("controller bootstrap failed; AMQP status=disconnected; retrying in 10s")
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
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

// Main parses flags, loads optional YAML, runs Work until the process receives SIGINT/SIGTERM, then shuts down HTTP.
func Main(cfg MainConfig) {
	if cfg.Work == nil {
		panic("collector.Main: nil Work")
	}
	if cfg.DefaultListen == "" {
		cfg.DefaultListen = ":8081"
	}
	if cfg.ConfigYAML == "" {
		panic("collector.Main: empty ConfigYAML")
	}

	listen := flag.String("listen", cfg.DefaultListen, "HTTP listen address for metrics and health")
	cfgDir := flag.String("configdir", ".", "directory containing YAML configuration files")
	controllerURL := flag.String("controller", "", "Controller Connect base URL for bootstrap; if empty, ICEHIVE_CONTROLLER_URL is used")
	flag.Parse()

	log := logrus.New()
	logging.ApplyEnvLogLevel(log)
	log.WithFields(logrus.Fields{
		"version": buildinfo.Version,
		"commit":  buildinfo.Commit,
		"date":    buildinfo.Date,
	}).Infof("%s collector-%s starting", common.ProjectName, cfg.ID)

	k, err := config.LoadOptionalYAML(log, *cfgDir, "collector-"+cfg.ID, cfg.ConfigYAML)
	if err != nil {
		log.WithError(err).Fatal("configuration")
	}

	ctrlBase := strings.TrimSpace(*controllerURL)
	if ctrlBase == "" {
		ctrlBase = strings.TrimSpace(os.Getenv("ICEHIVE_CONTROLLER_URL"))
	}
	if ctrlBase == "" {
		ctrlBase = "http://localhost:8080"
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var boot *bootstrap.WorkerRuntime
	kind := strings.TrimSpace(cfg.WorkerKind)
	if kind == "" {
		kind = "collector"
	}
	wid := strings.TrimSpace(cfg.WorkerID)
	if wid == "" {
		wid = cfg.ID
	}
	b, err := fetchBootstrapWithRetry(ctx, log, ctrlBase, kind, wid)
	if err != nil {
		log.WithError(err).Fatal("controller bootstrap")
	}
	amqpClient, err := waitForAMQPConnect(ctx, log, b, "ih_collector_"+cfg.ID)
	if err != nil {
		log.WithError(err).Fatal("amqp connect")
	}
	amqpClient.StartHeartbeatPublisher(ctx, "collector-"+cfg.ID, 10*time.Second)
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
	log.Infof("collector-%s listening on %s", cfg.ID, listenAddr)

	workErr := cfg.Work(ctx, k, log, boot, amqpClient, ctrlBase)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Fatal("graceful shutdown")
	}
	if workErr != nil {
		log.WithError(workErr).Fatal("collector work")
	}
}
