APP      := treesheild-newsbot
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build build-linux run preview once clean

build:
	go build -ldflags "-s -w" -o $(APP) .

# Сборка под Linux amd64 (типичный VPS)
build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o $(APP) .

run: build
	./$(APP)

preview: build
	./$(APP) -preview

once: build
	./$(APP) -run-once

clean:
	rm -f $(APP)
