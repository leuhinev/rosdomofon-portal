.PHONY: build run migrate-up migrate-down docker-build docker-up clean

build:
	go build -ldflags="-s -w" -o bin/portal .

run:
	go run .

migrate-up:
	mysql -h localhost -u root -ppassword portal < migrations/001_create_owners_table.up.sql
	mysql -h localhost -u root -ppassword portal < migrations/002_create_cars_table.up.sql
	mysql -h localhost -u root -ppassword portal < migrations/003_create_keys_table.up.sql

migrate-down:
	mysql -h localhost -u root -ppassword portal < migrations/003_create_keys_table.down.sql
	mysql -h localhost -u root -ppassword portal < migrations/002_create_cars_table.down.sql
	mysql -h localhost -u root -ppassword portal < migrations/001_create_owners_table.down.sql

docker-build:
	docker build -t rosdomofon-portal .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

clean:
	rm -rf bin/