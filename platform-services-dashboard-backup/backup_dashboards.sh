#!/usr/bin/env bash
set -euo pipefail

API_ENDPOINT="${API_ENDPOINT:-https://app.sysdigcloud.com/api}"
cd "$(dirname "$0")"
OUTPUT_DIR="./dashboards"

if [[ -z "${TOKEN:-}" ]]; then
  if [[ ! -f "$HOME/.sysdig_metrics_token" ]]; then
    echo "TOKEN is not set and ~/.sysdig_metrics_token was not found" >&2
    exit 1
  fi
  TOKEN=$(tr -d '\r\n' < "$HOME/.sysdig_metrics_token")
fi

command -v curl >/dev/null || exit 1
command -v jq >/dev/null || exit 1
mkdir -p "$OUTPUT_DIR"

comparable_json() {
  jq -S '
    del(.dashboard.lastAccessedOnByCurrentUser)
    | walk(if type == "object" then del(.documentTimestamp) else . end)
    | if .dashboard.permissions then .dashboard.permissions |= sort else . end
  ' "$1"
}

dashboards=$(curl -fsS -H "Authorization: Bearer $TOKEN" "$API_ENDPOINT/v3/dashboards")
jq -e '.dashboards | type == "array"' >/dev/null <<<"$dashboards"

while IFS=$'\t' read -r id name; do
  filename=$(sed -E 's/[^A-Za-z0-9]+/-/g; s/^-+//; s/-+$//' <<<"$name")
  [[ -n "$filename" ]] || { echo "Cannot create a filename for dashboard $id" >&2; exit 1; }
  destination="$OUTPUT_DIR/$filename.json"

  # Different dashboard names can normalize to the same filename. Never overwrite one silently.
  if [[ -f "$destination" ]] && [[ $(jq -r '.dashboard.id // empty' "$destination") != "$id" ]]; then
    echo "Filename collision: $destination" >&2
    exit 1
  fi

  # Validate the response before replacing the last known-good backup.
  temporary="$destination.tmp"
  curl -fsS -H "Authorization: Bearer $TOKEN" "$API_ENDPOINT/v3/dashboards/$id" | jq -e . > "$temporary"

  # Sysdig refreshes access and metric metadata timestamps on every read.
  if [[ -f "$destination" ]] && diff -q <(comparable_json "$destination") <(comparable_json "$temporary") >/dev/null; then
    rm "$temporary"
    echo "$id  $name -> unchanged"
    continue
  fi

  mv "$temporary" "$destination"
  echo "$id  $name -> dashboards/$filename.json"
done < <(jq -r '.dashboards[] | [.id, .name] | @tsv' <<<"$dashboards")
