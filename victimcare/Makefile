# Victim Care — development helpers
#
#   make run     start the server on http://localhost:8080
#   make test    run the test suite
#   make build   build the binary into bin/
#   make clean   remove build output and runtime data

.PHONY: run test build vet fmt clean

run:
	go run .

test:
	go test ./...

build:
	go build -o bin/victimcare .

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -rf bin data
