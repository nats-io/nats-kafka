# FIXME: Need to figure out how to generate certs with SANs.
# https://golang.org/doc/go1.15#commonname
export GODEBUG=x509ignoreCN=0

goSrc := $(shell find . -name "*.go")

nats-kafka: $(goSrc)
	go build -o $@

.PHONY: build
build: nats-kafka

.PHONY: install
install: nats-kafka
	mv $< $(shell go env GOPATH)/bin

.PHONY: install-tools
install-tools:
	cd $(HOME) && go get honnef.co/go/tools/cmd/staticcheck
	cd $(HOME) && go get github.com/client9/misspell/cmd/misspell
	cd $(HOME) && go get golang.org/x/tools/cmd/goimports

.PHONY: lint
lint:
	[ -z $$(gofmt -s -l $(goSrc)) ]
	[ -z $$(goimports -l $(goSrc)) ]
	misspell -locale US .
	go vet ./...
	staticcheck ./...

.PHONY: test
test:
	bash -e -c "trap 'trap - SIGINT ERR EXIT; $(MAKE) teardown-docker-test' SIGINT ERR EXIT; \
		$(MAKE) setup-docker-test && $(MAKE) run-test"

.PHONY: test-failfast
test-failfast:
	bash -e -c "trap 'trap - SIGINT ERR EXIT; $(MAKE) teardown-docker-test' SIGINT ERR EXIT; \
		$(MAKE) setup-docker-test && $(MAKE) run-test-failfast"

.PHONY: test-cover
test-cover:
	bash -e -c "trap 'trap - SIGINT ERR EXIT; $(MAKE) teardown-docker-test' SIGINT ERR EXIT; \
		$(MAKE) setup-docker-test && $(MAKE) run-test-cover"

.PHONY: setup-docker-test
setup-docker-test:
	docker-compose -p nats_kafka_test -f resources/test_servers.yml up -d
	scripts/wait_for_containers.sh

.PHONY: teardown-docker-test
teardown-docker-test:
	docker-compose -p nats_kafka_test -f resources/test_servers.yml down

.PHONY: run-test
run-test:
	# Running with -short to avoid flaky tests.
	go test -v -timeout 5m -short -race ./...

.PHONY: run-test-failfast
run-test-failfast:
	go test -timeout 5m -failfast ./...

.PHONY: run-test-cover
run-test-cover:
	go test -timeout 5m -coverpkg=./... -coverprofile=cover.out ./...
	go tool cover -html=cover.out
	rm cover.out
