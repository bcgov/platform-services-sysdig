#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 || ( $# -eq 2 && "$2" != "--apply" ) ]]; then
  echo "Usage: $0 <dashboard.json> [--apply]" >&2
  exit 1
fi

dashboard_file="$1"
[[ "$dashboard_file" = /* ]] || dashboard_file="$PWD/$dashboard_file"
[[ -f "$dashboard_file" ]] || { echo "File not found: $dashboard_file" >&2; exit 1; }

API_ENDPOINT="${API_ENDPOINT:-https://app.sysdigcloud.com/api}"
cd "$(dirname "$0")"

if [[ -z "${TOKEN:-}" ]]; then
  if [[ ! -f "$HOME/.sysdig_metrics_token" ]]; then
    echo "TOKEN is not set and ~/.sysdig_metrics_token was not found" >&2
    exit 1
  fi
  TOKEN=$(tr -d '\r\n' < "$HOME/.sysdig_metrics_token")
fi

jq -e '.dashboard.id | type == "number"' "$dashboard_file" >/dev/null
jq -e '.dashboard.name | type == "string" and length > 0' "$dashboard_file" >/dev/null
jq -e '.dashboard.panels | type == "array"' "$dashboard_file" >/dev/null

dashboard_id=$(jq -r '.dashboard.id' "$dashboard_file")
dashboard_name=$(jq -r '.dashboard.name' "$dashboard_file")
remote_file=$(mktemp)
response_file=$(mktemp)
payload_file=$(mktemp)
verify_file=$(mktemp)
trap 'rm -f "$remote_file" "$response_file" "$payload_file" "$verify_file"' EXIT

comparable_json() {
  jq -S '
    .dashboard |= with_entries(select(
      .key as $key
      | ["description", "eventDisplaySettings", "group", "id", "layout", "minInterval", "name", "panels", "schema", "scopeExpressionList", "sharingSettings"]
      | index($key)
    ))
    | walk(if type == "object" then del(.documentTimestamp) else . end)
  ' "$1"
}

status=$(curl -sS -o "$remote_file" -w '%{http_code}' \
  -H "Authorization: Bearer $TOKEN" \
  "$API_ENDPOINT/v3/dashboards/$dashboard_id")

if [[ "$status" == "404" ]]; then
  echo "Dashboard $dashboard_id no longer exists"

  if [[ "${2:-}" != "--apply" ]]; then
    echo "Dry run only. Add --apply to recreate it."
    exit 0
  fi

  # A recreated dashboard gets a new Sysdig ID. Server-owned fields from the old
  # dashboard must not be included in the create request.
  jq '
    .dashboard |= with_entries(select(
      .key as $key
      | ["description", "eventDisplaySettings", "group", "layout", "minInterval", "name", "panels", "public", "schema", "scopeExpressionList", "shared", "sharingSettings", "teamId"]
      | index($key)
    ))
  ' "$dashboard_file" > "$payload_file"

  status=$(curl -sS -o "$response_file" -w '%{http_code}' -X POST \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    --data-binary "@$payload_file" \
    "$API_ENDPOINT/v3/dashboards")

  if [[ "$status" != "200" && "$status" != "201" ]]; then
    echo "Recreate failed (HTTP $status)" >&2
    jq . "$response_file" 2>/dev/null || true
    exit 1
  fi

  new_dashboard_id=$(jq -r '.dashboard.id // empty' "$response_file")
  [[ "$new_dashboard_id" =~ ^[0-9]+$ ]] || { echo "Sysdig did not return a new dashboard ID" >&2; exit 1; }

  curl -fsS -H "Authorization: Bearer $TOKEN" \
    "$API_ENDPOINT/v3/dashboards/$new_dashboard_id" > "$verify_file"
  jq --argjson id "$new_dashboard_id" '.dashboard.id = $id' "$dashboard_file" > "$payload_file"

  if ! diff -q <(comparable_json "$verify_file") <(comparable_json "$payload_file") >/dev/null; then
    echo "Dashboard was created, but its content does not match the backup" >&2
    echo "New dashboard ID: $new_dashboard_id" >&2
    exit 1
  fi

  # Keep the backup pointed at the newly-created dashboard for future updates.
  jq . "$verify_file" > "$dashboard_file"
  echo "Recreated $dashboard_name ($dashboard_id -> $new_dashboard_id)"
  exit 0
fi
if [[ "$status" != "200" ]]; then
  echo "Failed to read dashboard $dashboard_id (HTTP $status)" >&2
  jq . "$remote_file" 2>/dev/null || true
  exit 1
fi

remote_id=$(jq -r '.dashboard.id // empty' "$remote_file")
remote_name=$(jq -r '.dashboard.name // empty' "$remote_file")
if [[ "$remote_id" != "$dashboard_id" ]]; then
  echo "Remote dashboard ID does not match $dashboard_id" >&2
  exit 1
fi

echo "Dashboard: $dashboard_name ($dashboard_id)"
echo "Remote:    $remote_name ($remote_id)"

if diff -q <(comparable_json "$remote_file") <(comparable_json "$dashboard_file") >/dev/null; then
  echo "No changes to restore"
  exit 0
fi

echo "Changes found:"
diff -u --label "Sysdig: $remote_name" --label "Backup: $dashboard_name" \
  <(comparable_json "$remote_file") <(comparable_json "$dashboard_file") || true

if [[ "${2:-}" != "--apply" ]]; then
  echo "Dry run only. Add --apply to update Sysdig."
  exit 0
fi

# Keep the exact remote response so a failed or unwanted update can be reversed.
mkdir -p restore-backups
safe_name=$(sed -E 's/[^A-Za-z0-9]+/-/g; s/^-+//; s/-+$//' <<<"$remote_name")
before_restore="restore-backups/$safe_name-$(date -u +%Y%m%dT%H%M%SZ).json"
cp "$remote_file" "$before_restore"

# GET responses contain server-owned fields. PUT accepts only the editable dashboard payload
# and expects the current remote version for conflict detection.
remote_version=$(jq -r '.dashboard.version' "$remote_file")
jq --argjson version "$remote_version" '
  .dashboard |= with_entries(select(
    .key as $key
    | ["description", "eventDisplaySettings", "group", "id", "layout", "minInterval", "name", "panels", "schema", "scopeExpressionList", "sharingSettings", "version"]
    | index($key)
  ))
  | .dashboard.version = $version
' "$dashboard_file" > "$payload_file"

status=$(curl -sS -o "$response_file" -w '%{http_code}' -X PUT \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  --data-binary "@$payload_file" \
  "$API_ENDPOINT/v3/dashboards/$dashboard_id")

if [[ "$status" != "200" ]]; then
  echo "Restore failed (HTTP $status). Original saved to $before_restore" >&2
  jq . "$response_file" 2>/dev/null || true
  exit 1
fi

jq -e --argjson id "$dashboard_id" '.dashboard.id == $id' "$response_file" >/dev/null

# A successful status is not enough: read it back and verify the editable fields.
curl -fsS -H "Authorization: Bearer $TOKEN" \
  "$API_ENDPOINT/v3/dashboards/$dashboard_id" > "$verify_file"
if ! diff -q <(comparable_json "$verify_file") <(comparable_json "$dashboard_file") >/dev/null; then
  echo "Sysdig returned success but the restored dashboard does not match the file" >&2
  echo "Original saved to $before_restore" >&2
  exit 1
fi

echo "Updated $dashboard_name ($dashboard_id)"
echo "Previous version: template-backup/$before_restore"
