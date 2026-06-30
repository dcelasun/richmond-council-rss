BINARY := richmond-council-rss

.PHONY: build test vet

build:
	go build -o $(BINARY) .

test:
	go test ./...

vet:
	go vet ./...
