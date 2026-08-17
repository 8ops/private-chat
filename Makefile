# Private Chat 构建/部署辅助
# 中国大陆构建请先：export GOPROXY=https://goproxy.cn,direct && export GOSUMDB=off

BINARY := private-chat
PKG    := ./cmd/server
GO     := go

.PHONY: build run test vet clean docker compose-up compose-down compose-logs

build:
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BINARY) $(PKG)

run: build
	./$(BINARY) -config configs/config.yaml

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

clean:
	rm -f $(BINARY)
	rm -rf data

docker:
	docker build -t private-chat:1.0.0 .

compose-up:
	docker compose up -d --build

compose-down:
	docker compose down

compose-logs:
	docker compose logs -f
