PROTO_SRC := api/proto/auth.proto
PROTO_OUT := internal/grpc/pb

generate: generate-sqlc generate-proto

generate-sqlc:
	sqlc generate -f sqlc.yaml

generate-proto:
	protoc \
		-I api/proto \
		--go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_SRC)

test:
	docker compose --profile test up --build \
      --abort-on-container-exit --exit-code-from tests \
      tests \
    && docker compose --profile test down -v --remove-orphans


run:
	docker compose up --build -d auth db

stop:
	docker compose down -v --remove-orphans

logs:
	docker compose logs -f auth db