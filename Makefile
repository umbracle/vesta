
protoc:
	protoc --go_out=. --go-grpc_out=. ./internal/server/proto/*.proto
