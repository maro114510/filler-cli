.PHONY: build test lint clean

build:
	go build -o filler-cli .

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f filler-cli
