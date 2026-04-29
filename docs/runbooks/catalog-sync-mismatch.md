# Catalog / media sync mismatch (machine clients)

Use when kiosks report stale catalogs, wrong media URLs, or alert **`AVFCatalogOrMediaGRPCErrorBurst`** fires.

## What the alert means

**Threshold:** `grpc_errors_total` rate for services matching **`MachineCatalogService`** or **`MachineMediaService`** above baseline (see `alerts.yml`).

Non-OK gRPC codes imply snapshot build failure, auth/credential gate, unavailable sale catalog service, or oversized response (`ResourceExhausted` on large manifests).

## How to query

```promql
sum(rate(grpc_errors_total{job="avf_api_metrics",service=~".*MachineCatalogService.*|.*MachineMediaService.*"}[5m])) by (service, method, grpc_code)
histogram_quantile(0.99, sum(rate(grpc_request_duration_seconds_bucket{job="avf_api_metrics",service=~".*MachineCatalogService.*"}[5m])) by (le))
```

**Logs:** gRPC interceptor + salecatalog build errors; search by **`machine_id`**, **`correlation_id`**.

**DB:** `machines`, `machine_current_snapshot`, planogram / assortment linkage for the org.

## Immediate mitigation

1. Confirm **Postgres** and **object store** healthy — presigned URLs depend on **`MediaStore`**.
2. If one org spikes, check recent admin catalog publishes and planogram versions.
3. If fleet-wide, check API CPU/memory and **`CAPACITY_MAX_MEDIA_MANIFEST_ENTRIES`**.

## Safe manual recovery

- Ask field to issue **GetCatalogSnapshot** with fresh request (clear local ETag/fingerprint cache) after backend fix.
- Do not hand-edit `catalog_version` fingerprints in DB without an admin publish path.

## Escalation

- Creative/content pipeline blocked: loop in retail ops owner.
- Sustained 5xx on catalog path: platform on-call per **`docs/runbooks/observability-alerts.md#api-high-5xx-rate`**.
