package main

//go:generate goapi-gen --package=spec --out ./internal/api/spec/journey.gen.spec.go ./internal/api/spec/journey.spec.json
//go:generate tern migrate --migrations ./internal/pgstore/migrations/ --config ./internal/pgstore/migrations/tern.conf
//go:generate sqlc generate -f ./internal/pgstore/sqlc.yaml
