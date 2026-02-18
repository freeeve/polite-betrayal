.PHONY: build run test test-integration lint fmt dev-up dev-down

build:
	cd api && go build -o bin/server ./cmd/server

run: build
	cd api && ./bin/server

test:
	cd api && go test ./...

lint:
	cd api && go vet ./...

fmt:
	cd api && gofmt -s -w .

test-integration:
	cd engine && cargo build --release
	cd api && REALPOLITIK_PATH=../engine/target/release/realpolitik go test ./internal/bot/ -tags=integration -run TestIntegration -v -count=1 -timeout=300s

dev-up:
	docker compose up -d

dev-down:
	docker compose down
