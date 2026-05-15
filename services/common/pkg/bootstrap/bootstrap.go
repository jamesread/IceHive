// Package bootstrap fetches worker runtime settings from the Controller over Connect.
package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	"github.com/icehive/icehive/services/common/pkg/amqpctl"
	icehivev1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/v1"
	"github.com/icehive/icehive/services/common/pkg/gen/icehive/v1/icehivev1connect"
)

// Params identifies the Controller endpoint and the calling worker.
type Params struct {
	BaseURL    string
	HTTPClient connect.HTTPClient
	Kind       string
	ID         string
}

// WorkerRuntime holds settings returned by the Controller for collector/persister startup.
type WorkerRuntime struct {
	AMQPURL                 string
	AMQPExchange            string
	RoutingKeyControlEvents string
	MySQLHost               string
	MySQLPort               int
	MySQLUser               string
	MySQLPassword           string
	MySQLDatabase           string
}

// Fetch calls Controller.WorkerBootstrap and maps the response into WorkerRuntime.
func Fetch(ctx context.Context, p Params) (*WorkerRuntime, error) {
	base := strings.TrimSpace(p.BaseURL)
	if base == "" {
		return nil, fmt.Errorf("bootstrap: empty controller base URL")
	}
	if strings.TrimSpace(p.Kind) == "" || strings.TrimSpace(p.ID) == "" {
		return nil, fmt.Errorf("bootstrap: kind and id are required")
	}
	hc := p.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}

	cli := icehivev1connect.NewControllerServiceClient(hc, base)
	getConfig := func(key string) (string, error) {
		resp, err := cli.GetConfig(ctx, connect.NewRequest(&icehivev1.GetConfigRequest{Key: key}))
		if err != nil {
			return "", fmt.Errorf("get config %q: %w", key, err)
		}
		if resp.Msg.GetVar() == nil {
			return "", nil
		}
		return strings.TrimSpace(resp.Msg.GetVar().GetValue()), nil
	}

	amqpURL, err := getConfig("amqp.url")
	if err != nil {
		return nil, err
	}
	amqpExchange, err := getConfig("amqp.exchange")
	if err != nil {
		return nil, err
	}
	amqpRK, err := getConfig("amqp.routing_key_control_events")
	if err != nil {
		return nil, err
	}

	// Backward-compatible AMQP URL assembly when amqp.url is not set.
	if amqpURL == "" {
		host, _ := getConfig("amqp.host")
		port, _ := getConfig("amqp.port")
		user, _ := getConfig("amqp.user")
		pass, _ := getConfig("amqp.password")
		vhost, _ := getConfig("amqp.vhost")
		if host != "" {
			if port == "" {
				port = "5672"
			}
			vhost = strings.TrimPrefix(vhost, "/")
			if vhost == "" {
				vhost = "/"
			}
			u := &url.URL{Scheme: "amqp", Host: host + ":" + port}
			if user != "" {
				u.User = url.UserPassword(user, pass)
			}
			if vhost == "/" {
				u.Path = "/"
			} else {
				u.Path = "/" + url.PathEscape(vhost)
			}
			amqpURL = u.String()
		}
	}

	wr := &WorkerRuntime{
		AMQPURL:                 amqpURL,
		AMQPExchange:            amqpExchange,
		RoutingKeyControlEvents: amqpRK,
	}
	// Persister sink DB settings are loaded via config-key reads from controller.
	if strings.EqualFold(strings.TrimSpace(p.Kind), "persister") {
		wr.MySQLHost, _ = getConfig("persister_mysql.host")
		port, _ := getConfig("persister_mysql.port")
		if port != "" {
			if n, convErr := strconv.Atoi(port); convErr == nil {
				wr.MySQLPort = n
			}
		}
		wr.MySQLUser, _ = getConfig("persister_mysql.user")
		wr.MySQLPassword, _ = getConfig("persister_mysql.password")
		wr.MySQLDatabase, _ = getConfig("persister_mysql.database")
	}
	if wr.AMQPExchange == "" {
		wr.AMQPExchange = amqpctl.DefaultControlExchange
	}
	if wr.RoutingKeyControlEvents == "" {
		wr.RoutingKeyControlEvents = amqpctl.RoutingKeyControlEvents
	}
	if wr.AMQPURL == "" {
		return nil, fmt.Errorf("bootstrap: controller returned empty amqp url")
	}
	return wr, nil
}
