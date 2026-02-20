module github.com/freeeve/polite-betrayal/api

go 1.25.6

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/gorilla/websocket v1.5.3
	github.com/lib/pq v1.11.2
	github.com/redis/go-redis/v9 v9.17.3
	github.com/rs/zerolog v1.34.0
	golang.org/x/oauth2 v0.35.0
)

replace github.com/advancedclimatesystems/gonnx => github.com/freeeve/gonnx v0.0.0-20260220175207-13af3a91603c

require (
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	golang.org/x/sys v0.12.0 // indirect
)
