APP := treesheild-newsbot

.PHONY: build build-linux preview run check test clean

build:
	go build -ldflags "-s -w" -o $(APP) .

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o $(APP) .

preview: build
	./$(APP) -preview

run: build
	./$(APP)

check: test
	go vet ./...

test:
	go test ./...

clean:
	rm -f $(APP)
