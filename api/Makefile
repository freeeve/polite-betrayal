test:
	go test ./... -count=1

test-integration: test-infra-up
	go test -tags integration ./... -count=1 -v
	$(MAKE) test-infra-down

test-infra-up:
	docker compose -f docker-compose.test.yml up -d --wait

test-infra-down:
	docker compose -f docker-compose.test.yml down -v

fmt:
	gofmt -s -w .

vet:
	go vet ./...

lint: fmt vet
