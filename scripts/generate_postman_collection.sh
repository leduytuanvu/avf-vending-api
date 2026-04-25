#!/usr/bin/env bash
# Emits native Postman collection + environment files under docs/postman/.
# Run from Makefile as "postman-generate" (after "swagger") or standalone after OpenAPI is built.
# OpenAPI is the source of truth at /swagger/doc.json; this does not convert doc.json to Postman format.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
PY="${PY:-python3}"

if [[ ! -f docs/swagger/swagger.json ]]; then
  echo "docs/swagger/swagger.json missing; generating OpenAPI"
  if command -v make >/dev/null 2>&1; then
    make swagger
  else
    "${PY}" tools/build_openapi.py
  fi
fi

"${PY}" tools/build_postman_collection.py
echo "OK: Postman artifacts under docs/postman/"
