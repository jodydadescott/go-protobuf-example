build:
	protoc \
		--go_out=. \
		--go_opt=paths=source_relative \
		--go-grpc_out=paths=source_relative,require_unimplemented_servers=false:. \
		*.proto
