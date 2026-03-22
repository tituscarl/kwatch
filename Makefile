BINARY=./bin/kwatch
VERSION?=0.1.0

.PHONY: build install clean tidy

build:
	go build -o $(BINARY) .
	
install:
	go install .

clean:
	rm -f $(BINARY)

tidy:
	go mod tidy
