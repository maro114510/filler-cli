.PHONY: build test lint clean results release-check release-snapshot tag

build:
	go build -o filler-cli .

test:
	go test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./...

lint:
	go vet ./...

clean:
	rm -f filler-cli

release-check:
	goreleaser check

release-snapshot:
	goreleaser release --snapshot --clean

tag:
	@[ -n "$(VERSION)" ] || (echo "Error: VERSION is required. Usage: make tag VERSION=v1.2.3" && exit 1)
	@echo "$(VERSION)" | grep -qE "^v[0-9]+\.[0-9]+\.[0-9]" || (echo "Error: VERSION must be semver (e.g., v1.2.3)" && exit 1)
	@git branch --show-current | grep -q "^main$$" || (echo "Error: must be on main branch to tag" && exit 1)
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

results: build
	@mkdir -p results
	./filler-cli analyze --format json --keep-filler-token 0 --output results/sample-a-filler0.result.json samples/sample_a.wav
	./filler-cli analyze --format json --keep-filler-token 1 --output results/sample-a-filler1.result.json samples/sample_a.wav
	./filler-cli analyze --format json --keep-filler-token 0 --output results/sample-b-filler0.result.json samples/sample_b.wav
	./filler-cli analyze --format json --keep-filler-token 1 --output results/sample-b-filler1.result.json samples/sample_b.wav
	./filler-cli analyze --format json --keep-filler-token 0 --output results/sample-c-filler0.result.json samples/sample_c.wav
	./filler-cli analyze --format json --keep-filler-token 1 --output results/sample-c-filler1.result.json samples/sample_c.wav
