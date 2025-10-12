include .env
air:
	- air

migrate-create:
	- migrate create -ext sql -dir internal/database/migration/ -seq $(name)
migrate-up:
	migrate -database="$(DATABASE_URL)" -path=internal/database/migration -lock-timeout=20 -verbose up
migrate-down:
	migrate -database="$(DATABASE_URL)" -path=internal/database/migration -lock-timeout=20 -verbose down 1
migrate-status:
	migrate -database="$(DATABASE_URL)" -path=internal/database/migration -lock-timeout=20 -verbose version
migrate-fresh:
	migrate -database="$(DATABASE_URL)" -path=internal/database/migration -lock-timeout=20 -verbose drop
	migrate -database="$(DATABASE_URL)" -path=internal/database/migration -lock-timeout=20 -verbose up
migrate-force:
	migrate -database="$(DATABASE_URL)" -path=internal/database/migration -lock-timeout=20 -verbose force $(version)
