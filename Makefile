include .env

# variables
MIGRATION_PATH = ./cmd/migrate/migrations
DB_ADDR = postgres://consultout:consultoutpassword@localhost:5432/consultoutdb?sslmode=disable

dockerDBCreate:  
	docker compose up --build

dockerDBRun:
	docker compose up -d

run:
	@go run ./cmd/api

.PHONY: migrations
migrations: 
# create db migration
	@migrate create -seq -ext sql -dir $(MIGRATION_PATH) $(filter-out $@,$(MAKECMDGOALS))

.PHONY: migrate-up
migrate-up:    
	@migrate -path=$(MIGRATION_PATH) -database=$(DB_ADDR) up 

.PHONY: migrate-up-to
migrate-up-to:
	@migrate -path=$(MIGRATION_PATH) -database=$(DB_ADDR) goto $(filter-out $@,$(MAKECMDGOALS))

migrate-up-prod:	
	@migrate -path=$(MIGRATION_PATH) -database=$(RENDER_EXTERNAL_DSN) up

# reset database 
.PHONY: migrate-reset
migrate-reset:
	@migrate -path=$(MIGRATION_PATH) -database=$(DB_ADDR) drop

.PHONY: migrate-reset-prod
migrate-reset-prod:
	@migrate -path=$(MIGRATION_PATH) -database=$(RENDER_EXTERNAL_DSN) drop

.PHONY: migrate-down
migrate-down:
	@migrate -path=$(MIGRATION_PATH) -database=$(DB_ADDR) down $(filter-out $@,$(MAKECMDGOALS))

migrate-down-to:
	@migrate -path=$(MIGRATION_PATH) -database=$(RENDER_EXTERNAL_DSN) goto $(filter-out $@,$(MAKECMDGOALS)) 

migrate-down-prod:
	@migrate -path=$(MIGRATION_PATH) -database=$(RENDER_EXTERNAL_DSN) down $(filter-out $@,$(MAKECMDGOALS))
	
migrate-force:
	@migrate -path=$(MIGRATION_PATH) -database=$(DB_ADDR) force $(filter-out $@,$(MAKECMDGOALS))

seed: 
	@go run cmd/migrate/seed/main.go


.PHONY: seed dockerDBCreate dockerDBRun

