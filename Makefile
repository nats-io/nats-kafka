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
	[ -z "$$(gofmt -s -l $(goSrc))" ]
	[ -z "$$(goimports -l $(goSrc))" ]
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

.PHONY: test-codecov
test-codecov:
	bash -e -c "trap 'trap - SIGINT ERR EXIT; $(MAKE) teardown-docker-test' SIGINT ERR EXIT; \
		$(MAKE) setup-docker-test && $(MAKE) run-test-codecov"

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
	go test -count=1 -timeout 5m -short -race ./...

.PHONY: run-test-failfast
run-test-failfast:
	go test -count=1 -timeout 5m -failfast ./...

.PHONY: run-test-cover
run-test-cover:
	go test -count=1 -timeout 5m -coverpkg=./... -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
	rm coverage.out

.PHONY: run-test-codecov
run-test-codecov:
	go test -v -count=1 -timeout 5m -short -race -covermode=atomic -coverprofile=coverage.out ./...
	./scripts/codecov_upload.sh
	rm coverage.out

nats-kafka.docker: $(goSrc)
	CGO_ENABLED=0 go build -o $@ -tags timetzdata \
		-ldflags "-X github.com/nats-io/nats-kafka/server/core.Version=$(VERSION)"

.PHONY: docker
docker: Dockerfile
ifneq ($(dtag),)
	docker build --tag natsio/nats-kafka:$(dtag) --build-arg VERSION=$(dtag) .
else
	# Missing dtag, try again. Example: make docker dtag=1.2.3
	exit 1
endif

.PHONY: dockerx
dockerx:
ifneq ($(ver),)
	# Ensure 'docker buildx ls' shows correct platforms.
	docker buildx build \
		--tag natsio/nats-kafka:$(ver) --tag natsio/nats-kafka:latest \
		--build-arg VERSION=$(ver) \
		--platform linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64/v8 \
		--push .
else
	# Missing version, try this.
	# make dockerx ver=1.2.3
	exit 1
endif

.PHONY: clean
clean:
	rm -f nats-kafka
	go clean --modcache
