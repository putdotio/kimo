NAME := kimo

build:
	go mod download
	go build -o $(NAME)

up:
	docker-compose stop
	docker-compose rm -fsv
	docker-compose build
	docker-compose up --scale kimo-server=5

lint:
	golangci-lint run
