.PHONY: build test lint clean

build:
	go build -o filler-cli .

test:
	go test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./...

lint:
	go vet ./...

clean:
	rm -f filler-cli
