
build: fmt check compile

fmt:
	misspell -locale US .
	gofmt -s -w *.go
	gofmt -s -w server/conf/*.go
	gofmt -s -w server/core/*.go
	gofmt -s -w server/logging/*.go
	goimports -w *.go
	goimports -w server/conf/*.go
	goimports -w server/core/*.go
	goimports -w server/logging/*.go

check:
	go vet ./...
	staticcheck ./...

update:
	go get -u honnef.co/go/tools/cmd/staticcheck
	go get -u github.com/client9/misspell/cmd/misspell
  
compile:
	go build ./...

install: build
	go install ./...

cover: test
	go tool cover -html=./coverage.out

test: check
	rm -rf ./cover.out
	@echo "Running kafka and zookeeper in docker..."
	docker-compose -p nats_kafka_test -f resources/test_servers.yml up -d
	@echo "Waiting kafka and zookeeper..."
	-scripts/wait_for_containers.sh
	@echo "Running tests..."
	-go test -race -coverpkg=./... -coverprofile=./coverage.out ./...
	@echo "Cleaning up..."
	docker-compose -p nats_kafka_test -f resources/test_servers.yml down

failfast:
	@echo "Running kafka and zookeeper in docker..."
	docker-compose -p nats_kafka_test -f resources/test_servers.yml up -d
	@echo "Waiting kafka and zookeeper..."
	-scripts/wait_for_containers.sh
	@echo "Running tests..."
	-go test --failfast ./...
	@echo "Cleaning up..."
	docker-compose -p nats_kafka_test -f resources/test_servers.yml down