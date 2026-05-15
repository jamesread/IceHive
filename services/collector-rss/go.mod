module github.com/icehive/icehive/services/collector-rss

go 1.25.0

require (
	connectrpc.com/connect v1.19.2
	github.com/icehive/icehive/services/common v0.0.0
	github.com/knadh/koanf/v2 v2.3.4
	github.com/mmcdole/gofeed v1.2.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.9.4
	golang.org/x/text v0.30.0
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/PuerkitoBio/goquery v1.8.0 // indirect
	github.com/andybalholm/cascadia v1.3.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/jamesread/golure v0.0.0-20250919212919-976d085a100c // indirect
	github.com/jamesread/httpauthshim v0.0.0-20260429075835-6727f3a27077 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/parsers/yaml v1.1.0 // indirect
	github.com/knadh/koanf/providers/file v1.2.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/mmcdole/goxpp v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/rabbitmq/amqp091-go v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.3 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
)

replace github.com/icehive/icehive/services/common => ../common
