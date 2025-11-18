.PHONY: build clean test tidy update

build:
	go build -o hy ./cmd/hy

clean:
	rm -f hy

test:
	go test ./...

tidy:
	go mod tidy

update:
	go get -u ./...
	go mod tidy

