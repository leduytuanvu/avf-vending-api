#!/usr/bin/env bash
# shellcheck shell=bash
# Redact secrets from JSON/text for evidence output.

prod_e2e_redact_file() {
  local in_file="$1"
  local out_file="$2"
  if ! command -v jq >/dev/null 2>&1; then
    cp "$in_file" "$out_file"
    return 0
  fi
  if jq -e . >/dev/null 2>&1 <"$in_file"; then
    jq '
      walk(
        if type == "object" then
          with_entries(
            if (.key | test("password|token|secret|authorization|refresh"; "i")) then
              .value = "<redacted>"
            else . end
          )
        else . end
      )
    ' "$in_file" >"$out_file" 2>/dev/null || cp "$in_file" "$out_file"
  else
    sed -E \
      -e 's/("password"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
      -e 's/("accessToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
      -e 's/("refreshToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
      -e 's/("machineToken"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
      -e 's/(Bearer[[:space:]]+)[A-Za-z0-9._-]+/\1<redacted>/g' \
      "$in_file" >"$out_file"
  fi
}

prod_e2e_redact_inline() {
  local s="$1"
  printf '%s' "$s" | sed -E \
    -e 's/("password"[[:space:]]*:[[:space:]]*")[^"]*"/\1<redacted>"/gi' \
    -e 's/(Bearer[[:space:]]+)[A-Za-z0-9._-]+/\1<redacted>/g'
}
