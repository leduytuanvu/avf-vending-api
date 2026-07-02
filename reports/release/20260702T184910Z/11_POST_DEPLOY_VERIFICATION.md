# Post-Deploy Verification

**UTC:** 20260702T193800Z  
**Deploy run:** [28616475685](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28616475685)

## HTTP checks

| Check | URL | Result |
|-------|-----|--------|
| Live | `GET https://api.ldtv.dev/health/live` | **200** |
| Ready | `GET https://api.ldtv.dev/health/ready` | **200** |
| Version | `GET https://api.ldtv.dev/version` | **200** — `git_sha=156fc468fa3c5fec7042e1f656f78b6ea94c2639` |
| OpenAPI | `GET https://api.ldtv.dev/swagger/doc.json` | **200** |

## TLS reachability (no commands sent)

| Target | Result |
|--------|--------|
| `mqtt.ldtv.dev:8883` | **reachable** |
| `machine-api.ldtv.dev:443` | **reachable** |

## Verdict

**POST_DEPLOY_VERIFICATION_PASS**
