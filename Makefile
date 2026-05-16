.PHONY: all build test lint clean proto frontend services docs integration-tests

all: proto services frontend

build: all

proto:
	$(MAKE) -wC protocol

services:
	$(MAKE) -wC services/common
	$(MAKE) -wC services/controller
	$(MAKE) -wC services/collector-github
	$(MAKE) -wC services/collector-azure
	$(MAKE) -wC services/collector-jmap
	$(MAKE) -wC services/collector-rss
	$(MAKE) -wC services/collector-testdata
	$(MAKE) -wC services/persister-yaml
	$(MAKE) -wC services/persister-mysql

frontend:
	$(MAKE) -wC frontend

test:
	$(MAKE) -wC services/common test
	$(MAKE) -wC services/controller test
	$(MAKE) -wC services/collector-github test
	$(MAKE) -wC services/collector-azure test
	$(MAKE) -wC services/collector-jmap test
	$(MAKE) -wC services/collector-rss test
	$(MAKE) -wC services/collector-testdata test
	$(MAKE) -wC services/persister-yaml test
	$(MAKE) -wC services/persister-mysql test
	$(MAKE) -wC frontend test

lint:
	$(MAKE) -wC services/common lint
	$(MAKE) -wC services/controller lint
	$(MAKE) -wC services/collector-github lint
	$(MAKE) -wC services/collector-azure lint
	$(MAKE) -wC services/collector-jmap lint
	$(MAKE) -wC services/collector-rss lint
	$(MAKE) -wC services/collector-testdata lint
	$(MAKE) -wC services/persister-yaml lint
	$(MAKE) -wC services/persister-mysql lint
	$(MAKE) -wC frontend lint

integration-tests:
	$(MAKE) -wC integration-tests

docs:
	$(MAKE) -wC docs

clean:
	$(MAKE) -wC services/common clean
	$(MAKE) -wC services/controller clean
	$(MAKE) -wC services/collector-github clean
	$(MAKE) -wC services/collector-azure clean
	$(MAKE) -wC services/collector-jmap clean
	$(MAKE) -wC services/collector-rss clean
	$(MAKE) -wC services/collector-testdata clean
	$(MAKE) -wC services/persister-yaml clean
	$(MAKE) -wC services/persister-mysql clean
	$(MAKE) -wC frontend clean
	$(MAKE) -wC docs clean
