# Regenerates marketing screenshots against a throwaway stack seeded with
# fictional data. Run `make` here; everything is torn down afterwards.

ROOT := $(abspath ../../..)
HERE := $(abspath .)
TMP := $(HERE)/tmp

CONTAINER ?= podman
MYSQL_CTR := icehive-mkt-mysql
RABBIT_CTR := icehive-mkt-rabbitmq

CONTROLLER_URL := http://127.0.0.1:18480
FRONTEND_PORT := 18400

DOCS_ASSETS := $(ROOT)/docs/content/assets/marketing
DOCS_IMAGES := home.png sources.png source-details.png services.png config.png

.PHONY: all build up seed shoot manifest publish down clean

all:
	@$(MAKE) build up seed shoot manifest publish; rc=$$?; $(MAKE) down; exit $$rc

build:
	$(MAKE) -C $(ROOT)/services/controller build
	$(MAKE) -C $(ROOT)/services/collector-testdata build
	$(MAKE) -C $(ROOT)/services/persister-mysql build
	[ -d $(ROOT)/frontend/node_modules ] || (cd $(ROOT)/frontend && npm ci)

up:
	mkdir -p $(TMP)/controller $(TMP)/collector-testdata $(TMP)/persister-mysql
	cp controller.yaml $(TMP)/controller/config.yaml
	ln -sfn $(ROOT)/services/controller/migrations $(TMP)/controller/migrations
	cp collector-testdata.yaml $(TMP)/collector-testdata/
	cp persister-mysql.yaml $(TMP)/persister-mysql/
	$(CONTAINER) run -d --rm --name $(MYSQL_CTR) -p 127.0.0.1:13306:3306 \
		-e MYSQL_ROOT_PASSWORD=root-demo -e MYSQL_DATABASE=icehive \
		-e MYSQL_USER=icehive -e MYSQL_PASSWORD=icehive-demo \
		-v $(HERE)/mysql-init.sql:/docker-entrypoint-initdb.d/init.sql:ro,Z \
		docker.io/library/mysql:8.4
	$(CONTAINER) run -d --rm --name $(RABBIT_CTR) -p 127.0.0.1:15682:5672 \
		docker.io/library/rabbitmq:4-alpine
	@echo "Waiting for MySQL..."
	@for i in $$(seq 120); do \
		$(CONTAINER) exec $(MYSQL_CTR) mysql -uicehive_sink -picehive-sink-demo -h127.0.0.1 -e 'SELECT 1' icehive_sink >/dev/null 2>&1 && exit 0; \
		sleep 1; done; echo "MySQL did not start"; exit 1
	@echo "Waiting for RabbitMQ..."
	@for i in $$(seq 120); do \
		$(CONTAINER) exec $(RABBIT_CTR) rabbitmq-diagnostics -q check_port_connectivity >/dev/null 2>&1 && exit 0; \
		sleep 1; done; echo "RabbitMQ did not start"; exit 1
	cd $(TMP)/controller && { $(ROOT)/services/controller/controller -configdir . > $(TMP)/controller.log 2>&1 & echo $$! > $(TMP)/controller.pid; }
	@for i in $$(seq 60); do curl -fsS -X POST -H 'Content-Type: application/json' -d '{}' \
		$(CONTROLLER_URL)/api/icehive.v1.ControllerService/Health >/dev/null 2>&1 && exit 0; sleep 1; done; \
		echo "Controller did not become healthy; see $(TMP)/controller.log"; exit 1

seed:
	./seed.sh $(CONTROLLER_URL)
	cd $(TMP)/collector-testdata && { ICEHIVE_CONTROLLER_URL=$(CONTROLLER_URL) $(ROOT)/services/collector-testdata/collector-testdata -configdir . > $(TMP)/collector-testdata.log 2>&1 & echo $$! > $(TMP)/collector-testdata.pid; }
	cd $(TMP)/persister-mysql && { ICEHIVE_CONTROLLER_URL=$(CONTROLLER_URL) $(ROOT)/services/persister-mysql/persister-mysql -configdir . > $(TMP)/persister-mysql.log 2>&1 & echo $$! > $(TMP)/persister-mysql.pid; }
	cd $(ROOT)/frontend && { ICEHIVE_CONTROLLER_PORT=$(CONTROLLER_URL) npx vite --host 127.0.0.1 --port $(FRONTEND_PORT) --strictPort > $(TMP)/frontend.log 2>&1 & echo $$! > $(TMP)/frontend.pid; }
	@echo "Waiting for workers to report healthy..."
	@for i in $$(seq 90); do \
		n=$$(curl -fsS -X POST -H 'Content-Type: application/json' -d '{}' \
			$(CONTROLLER_URL)/api/icehive.v1.ControllerService/ListServices 2>/dev/null | grep -o '"healthy"' | wc -l); \
		[ "$$n" -ge 2 ] && exit 0; sleep 1; done; echo "Workers did not become healthy"; exit 1
	@echo "Enqueueing a collection run..."
	@for i in $$(seq 30); do curl -fsS -X POST -H 'Content-Type: application/json' -d '{"collectionSourceId":"demo-animals"}' \
		$(CONTROLLER_URL)/api/icehive.v1.ControllerService/EnqueueCollectionRequest >/dev/null 2>&1 && break; sleep 1; done
	$(CONTAINER) exec -i $(MYSQL_CTR) mysql -uicehive -picehive-demo icehive < seed/run-history.sql
	@for i in $$(seq 60); do curl -fsS http://127.0.0.1:$(FRONTEND_PORT)/ >/dev/null 2>&1 && break; sleep 1; done
	@echo "Letting a few batches flow through..."
	sleep 15

shoot:
	repo-helper screenshot --config screenshots.ini

manifest:
	./generate-manifest.py

publish:
	mkdir -p $(DOCS_ASSETS)
	cp $(DOCS_IMAGES) $(DOCS_ASSETS)/

down:
	-@for p in frontend persister-mysql collector-testdata controller; do \
		[ -f $(TMP)/$$p.pid ] && pkill -P $$(cat $(TMP)/$$p.pid) 2>/dev/null; \
		[ -f $(TMP)/$$p.pid ] && kill $$(cat $(TMP)/$$p.pid) 2>/dev/null; \
		rm -f $(TMP)/$$p.pid; done
	-@$(CONTAINER) rm -f $(MYSQL_CTR) $(RABBIT_CTR) >/dev/null 2>&1

clean: down
	rm -rf $(TMP)
