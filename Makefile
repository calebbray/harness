.PHONY: run build test release clean

run:
	HARNESS_DATA_DIR=. go run .

build:
	go build -o harness .

test:
	go test -v ./...

clean:
	rm -f harness

release: test
	./scripts/release.sh
