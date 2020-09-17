NAME := kimo

build:
	go mod download
	go build -o $(NAME)

up:
	docker-compose stop
	docker-compose rm -fsv
	docker-compose build mysql
	docker-compose build kimo
	docker-compose build kimo-daemon
	docker-compose build kimo-server
	docker-compose build tcpproxy
	docker-compose up --scale kimo-daemon=5

lint:
	golangci-lint run
