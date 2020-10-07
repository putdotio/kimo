NAME := kimo
GOPATH=$(shell go env GOPATH)
export GOPATH

build:
	go mod download
	go get github.com/rakyll/statik
	$(GOPATH)/bin/statik -src=./server/static -include='*.html'
	go install
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
