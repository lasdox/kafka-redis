.PHONY: run tidy build

run: tidy build
	go run .

tidy:
	go mod tidy

build:
	go build -o kafkaredis .