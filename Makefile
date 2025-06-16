include .env
air:
	- air
migration-create:
	- migrate create -ext sql -dir internal/database/migration/ -seq $(name)
migrate-up:
	migrate -database="$(DATABASE_URL)" -path=internal/database/migration -lock-timeout=20  -verbose up
migrate-down:
	migrate -database="$(DATABASE_URL)" -path=internal/database/migration -lock-timeout=20 -verbose down