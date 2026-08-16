# costGuard

A small Go application for estimating cloud costs from the command line and HTTP API.

The project is designed to support:

- AWS
- Azure
- GCP

Pricing calculations are being built behind provider-specific adapters so the presentation layer does not depend on a single cloud provider.

## Current status

The HTTP presentation layer is available. Pricing calculation and live provider catalogs are still in progress.

The estimate endpoint currently returns `503 Service Unavailable` until a pricing catalog and Domain Layer calculator are connected.

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

## OpenAPI

The API uses Swaggo annotations.

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

