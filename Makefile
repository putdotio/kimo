NAME := kimo

build:
	go mod download
	go build -o $(NAME)

up:
	docker-compose stop
	docker-compose rm -fsv
	docker-compose build
	docker-compose up

lint:
	golangci-lint run
