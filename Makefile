
protoc:
	protoc --go_out=. --go-grpc_out=. ./internal/server/proto/*.proto
	protoc --go_out=. --go-grpc_out=. ./internal/client/runner/structs/*.proto
