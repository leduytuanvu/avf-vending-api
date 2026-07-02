# CI Status — avf-vending-api main release

**UTC:** 20260702T192600Z  
**Main SHA:** `156fc468fa3c5fec7042e1f656f78b6ea94c2639`

## develop → main promotion

| Step | Run ID | Conclusion |
|------|--------|------------|
| PR #392 → develop | merge `78d6ba89` | merged |
| PR #393 develop → main | merge `156fc468` | merged |
| CI (main push) | [28615428060](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28615428060) | **success** |
| Security (main push) | [28615428781](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28615428781) | **success** |
| Enterprise release verification | [28615428052](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28615428052) | **success** |

## Build and Security Release chain (main)

| Step | Run ID | Conclusion |
|------|--------|------------|
| Build and Push Images | [28615687682](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28615687682) | **success** |
| Security Release | [28615969226](https://github.com/leduytuanvu/avf-vending-api/actions/runs/28615969226) | **success** (verdict: pass) |

## Image digests (build 28615687682)

- **app:** `ghcr.io/leduytuanvu/avf-vending-api@sha256:17785fabd82a7cfe8c4e9dcc7d8f2693a03ff186a5294d69a97c7b5e890fddae`
- **goose:** `ghcr.io/leduytuanvu/avf-vending-api-goose@sha256:ab9abcb8cce2fe7d2136305da4603c5e2ec4c55eaa098aaf26bc11b31587aae6`

## Staging evidence

No successful **Staging Deployment Contract** run found for digest `156fc468`. Production deploy uses documented bypass (`allow_missing_staging_evidence`) per prior operator releases.

## Verdict

**CI_BUILD_SECURITY_RELEASE_PASS** — ready for manual Deploy Production.
