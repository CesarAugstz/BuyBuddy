
.PHONY: run build test clean install dev

run:
	go run main.go

dev:
	air

build:
	go build -o bin/easybuy-api main.go

test:
	go test ./...

clean:
	rm -rf bin/
	rm -rf tmp/

install:
	go mod download
	go mod tidy

docker-build:
	docker build -t easybuy-api .

docker-run:
	docker run -p 8080:8080 --env-file .env easybuy-api
