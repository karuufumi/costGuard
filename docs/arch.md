
# Architecture Documentation for costGuard

This app will leverage a simple 3-tier layered architecture style

Presentation (CLI) Layer --> Domain Layer --> Persistence Layer

## Presentation Layer

## Domain Layer

## Persistence Layer

## HTTP API surface

The HTTP API is a secondary Presentation Layer adapter. The CLI remains the
primary user interface, and both interfaces should call the same Domain Layer
when estimate calculation is implemented.

| Method | Route | Purpose | Status |
| --- | --- | --- | --- |
| `GET` | `/healthz` | Container/process health check | Implemented |
| `GET` | `/v1/providers/{provider}/services` | List planned provider services | Implemented |
| `GET` | `/v1/providers/{provider}/regions` | List known provider regions | Implemented |
| `GET` | `/v1/catalog` | Report the embedded catalog status | Implemented |
| `POST` | `/v1/estimates` | Calculate an estimate from structured input | AWS EC2 `t3.micro` in `us-east-1` implemented |

The first calculator uses a small versioned embedded catalog. Unsupported
products fail clearly rather than returning invented prices. Its only completed
product is AWS EC2 Linux shared-tenancy on-demand `t3.micro` in `us-east-1`.
Data transfer, EBS, public IPv4, taxes, support, Savings Plans, Reserved
Instances, Spot, and operating-system licensing are excluded.

## API patterns applied

- Versioned resource paths use the `/v1` prefix.
- Standard HTTP methods and status codes are used.
- JSON errors use a consistent Problem Details-style response.
- Request bodies reject unknown fields and are limited to 1 MiB.
- The HTTP server uses read, write, header, and idle timeouts.
- Discovery endpoints expose stable identifiers such as `ec2` and
  `ap-southeast-1`, rather than implementation-specific names.
- Swaggo annotations document the HTTP handlers for later OpenAPI generation.
- The API uses transport types that can be mapped into domain types later,
  avoiding direct coupling to persistence or AWS SDK structures.

## Deferred API features

Pagination, authentication, rate limiting, live catalog refresh, saved estimate
resources, idempotency keys, and asynchronous operations are deferred until the
domain and actual usage requirements justify them.
## Unified multi-cloud architecture

costGuard supports AWS, Azure, and GCP through provider-specific pricing adapters behind a common Domain Layer. The provider identifier is part of every estimate contract from the beginning.

```text
CLI/HTTP Presentation -> Domain Calculator -> Pricing Provider Adapter
                                                   |-- AWS
                                                   |-- Azure
                                                   `-- GCP
```

Canonical HTTP API:

```text
GET   /healthz
GET   /v1/providers
GET   /v1/providers/{provider}/services
GET   /v1/providers/{provider}/regions
GET   /v1/catalog
GET   /v1/config
PUT   /v1/config
PATCH /v1/config
DELETE /v1/config
GET   /v1/account
POST  /v1/estimates
```

`PUT /v1/config` replaces the complete preferences document. `PATCH /v1/config`
updates selected fields. `DELETE /v1/config` resets preferences and clears only
costGuard-owned cached state. It must not delete cloud credentials, which are
managed by each provider's credential chain or identity system.

The first implementation may complete one provider/service path first, but must retain the provider-neutral contract and adapter boundary so Azure and GCP do not require a rewrite.
