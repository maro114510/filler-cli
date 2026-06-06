.PHONY: build test lint clean results

build:
	go build -o filler-cli .

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f filler-cli

results: build
	@mkdir -p results
	./filler-cli analyze --format json --keep-filler-token 0 --output results/sample-a-filler0.result.json samples/sample_a.wav
	./filler-cli analyze --format json --keep-filler-token 1 --output results/sample-a-filler1.result.json samples/sample_a.wav
	./filler-cli analyze --format json --keep-filler-token 0 --output results/sample-b-filler0.result.json samples/sample_b.wav
	./filler-cli analyze --format json --keep-filler-token 1 --output results/sample-b-filler1.result.json samples/sample_b.wav
	./filler-cli analyze --format json --keep-filler-token 0 --output results/sample-c-filler0.result.json samples/sample_c.wav
	./filler-cli analyze --format json --keep-filler-token 1 --output results/sample-c-filler1.result.json samples/sample_c.wav
