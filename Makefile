.PHONY: build run test clean dev sqlc

BINARY=chat-empleados
DB_FILE=chat.db 

build: 
	go build -o $(BINARY) .

run: build
	./$(BINARY)

dev: 
	go run .

test: 
	go test ./... -v

check:
	go build ./...
	go vet ./...

sqlc:
	sqlc generate 

clean:
	rm -f $(BINARY)

reset-db:
	rm -f $(DB_FILE)
	sqlite3 $(DB_FILE) < schema.sql 

