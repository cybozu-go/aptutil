test:
	go test -count 1 -race -v ./... -coverprofile cover.out

build:
	go build ./...
