NAME := kimo
GOPATH=$(shell go env GOPATH)
export GOPATH

build:
	go mod download
	go get github.com/rakyll/statik
	$(GOPATH)/bin/statik -src=./server/static -include='*.html'
	go install
build-dependencies:
	docker-compose stop
	docker-compose rm -fsv
	docker-compose build mysql
	docker-compose build kimo
up:
	docker-compose build kimo
	docker-compose up --build kimo-server kimo-agent
lint:
	golangci-lint run
