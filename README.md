# costGuard

A small Go application for estimating cloud costs through a private HTTP API.

The project is designed to support:

- AWS
- Azure
- GCP

Pricing calculations are being built behind provider-specific adapters so the presentation layer does not depend on a single cloud provider.

## Current status

The first deterministic pricing path is available: AWS EC2 Linux shared-tenancy,
on-demand `t3.micro` in `us-east-1`. It uses an embedded catalog snapshot
(`2026-08-18.1`) so results are reproducible offline.

All other provider, service, region, or instance combinations remain unsupported
until their catalog entries and calculation rules are implemented. Live pricing
refresh is not implemented.

## Run locally

Requirements:

- Go 1.26 or newer

Start the API:

```bash
go run .
```

The server listens on port `8000` by default. Set another port with:

```bash
PORT=9000 go run .
```

## API

Health check:

```bash
curl http://localhost:8000/healthz
```

List providers:

```bash
curl http://localhost:8000/v1/providers
```

List services for a provider:

```bash
curl http://localhost:8000/v1/providers/aws/services
```

Read local preferences:

```bash
curl http://localhost:8000/v1/config
```

Update one preference:

```bash
curl -X PATCH http://localhost:8000/v1/config \
  -H 'Content-Type: application/json' \
  -d '{"currency":"EUR"}'
```

Reset saved preferences:

```bash
curl -X DELETE http://localhost:8000/v1/config
```

Configuration stores preferences only. Cloud credentials are never accepted or stored by costGuard.

Create an EC2 estimate:

```bash
curl -X POST http://localhost:8000/v1/estimates \
  -H 'Content-Type: application/json' \
  -d '{"provider":"aws","service":"ec2","region":"us-east-1","usage":{"instance":"t3.micro","hours":730}}'
```

The result includes hourly, daily, monthly, and annual totals, the catalog
version, assumptions, and omitted cost dimensions. It is an estimate, not a bill.

## OpenAPI

The API uses Swaggo annotations.

Start the app and open the interactive API documentation at:

```text
http://localhost:8000/docs/index.html
```

Generate the OpenAPI files with:

```bash
go generate .
```

Generated files:

- `docs/swagger.json`
- `docs/swagger.yaml`

## Architecture

The project uses a simple layered design:

```text
CLI or HTTP presentation
          |
      Domain layer
          |
 Pricing and persistence adapters
```

The presentation layer handles requests, validation, responses, and documentation. It does not calculate prices or access cloud provider APIs directly.

## Development checks

```bash
gofmt -w .
go test ./...
go vet ./...
go build ./...
```
