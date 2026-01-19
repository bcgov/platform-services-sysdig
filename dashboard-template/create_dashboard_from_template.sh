#!/usr/bin/env bash
set -eo pipefail

# Script to create a new dashboard from a template in a specific team
# Usage: ./create_dashboard_from_template.sh <api_token> <template_file> <target_team_id>

API_ENDPOINT="https://app.sysdigcloud.com/api"
PLATFORM_API_ENDPOINT="https://api.us1.sysdig.com"

#--- check arguments -------------------------------------------------------
if [ $# -ne 3 ]; then
  echo "Usage: $0 <api_token> <template_file> <target_team_id>"
  echo "Example: $0 'your-api-token' template/dashboard_template_resources_quota_approve_dashboard.json.j2 50396"
  echo ""
  echo "To find your API token:"
  echo "1. Go to https://app.sysdigcloud.com/#/settings/user"
  echo "2. Look under 'Sysdig Monitor API Token'"
  echo ""
  echo "To find team IDs:"
  echo "curl -X GET -H \"Authorization: Bearer <API_TOKEN>\" https://app.sysdigcloud.com/api/v3/teams"
  exit 1
fi

TOKEN="$1"
TEMPLATE_FILE="$2"
TARGET_TEAM_ID="$3"
export TOKEN

#--- validate template file exists ------------------------------------------
if [ ! -f "$TEMPLATE_FILE" ]; then
  echo "!!!!Template file '$TEMPLATE_FILE' not found"
  exit 1
fi

echo "Creating dashboard from template: $TEMPLATE_FILE"
echo "Target team ID: $TARGET_TEAM_ID"

#--- validate team ID -----------------------------------------------------
if ! [[ "$TARGET_TEAM_ID" =~ ^[0-9]+$ ]]; then
  echo "!!!!Invalid team ID: $TARGET_TEAM_ID (must be a number)"
  exit 1
fi

echo "✅ Using team ID: $TARGET_TEAM_ID"

#--- load and modify template ----------------------------------------------
echo "Loading and modifying template..."

# Read the template
template_json=$(cat "$TEMPLATE_FILE")


# Modify the template with team-specific information (template already has dashboard wrapper)
modified_template=$(echo "$template_json" | jq --argjson team_id "$TARGET_TEAM_ID" \
  '.dashboard.teamId = $team_id | .dashboard.userId = null | .dashboard.sharingSettings = []')

if [ -z "$modified_template" ] || [ "$modified_template" = "null" ]; then
  echo "!!!!Failed to modify template"
  exit 1
fi

echo "✅ Template modified for team ID: $TARGET_TEAM_ID"

#--- create the dashboard --------------------------------------------------
echo "Creating dashboard..."

create_response=$(curl -sS -X POST \
  "$API_ENDPOINT/v3/dashboards" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -H "x-sysdig-public-notation: true" \
  --data "$modified_template")


if [ $? -ne 0 ]; then
  echo "❌ Failed to create dashboard"
  echo "Response: $create_response"
  exit 1
fi

# Check if creation was successful
dashboard_id=$(echo "$create_response" | jq -r '.dashboard.id // empty')
if [ -z "$dashboard_id" ]; then
  echo "❌ Dashboard creation failed"
  echo "Response: $create_response"
  exit 1
fi

dashboard_name=$(echo "$create_response" | jq -r '.dashboard.name')
echo "✅ Dashboard created successfully!"
echo "📊 Dashboard: $dashboard_name (ID: $dashboard_id)"
echo "👥 Team ID: $TARGET_TEAM_ID"
