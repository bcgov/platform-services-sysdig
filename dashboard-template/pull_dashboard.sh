#!/usr/bin/env bash
set -eo pipefail

# Script to pull a dashboard from Sysdig and export it as a template
# Uses the Dashboard Listing API to find dashboards by name
# Usage: ./pull_dashboard.sh <dashboard_name>

API_ENDPOINT="https://app.sysdigcloud.com/api"

#--- ensure TOKEN is set ---------------------------------------------------
if [ -z "$TOKEN" ]; then
  if [ -f ~/.sysdig_metrics_token ]; then
    TOKEN=$(<~/.sysdig_metrics_token)
  else
    echo "Error: API token not found in ~/.sysdig_metrics_token"
    echo "Please create this file with your Sysdig API token"
    exit 1
  fi
fi
export TOKEN

#--- check arguments -------------------------------------------------------
if [ $# -ne 1 ]; then
  echo "Usage: $0 <dashboard_name>"
  echo "Example: $0 'Resource Allocation Dashboard'"
  exit 1
fi

DASHBOARD_NAME="$1"

echo "🔍 Looking for dashboard: '$DASHBOARD_NAME'"

#--- get list of dashboards ------------------------------------------------
echo "📊 Getting list of dashboards..."
dashboards_response=$(curl -sS -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "$API_ENDPOINT/v3/dashboards")

if [ $? -ne 0 ]; then
  echo "❌ Failed to get dashboards list"
  exit 1
fi

# Find matching dashboard
matching_dashboard=$(echo "$dashboards_response" | jq -r --arg dashboard_name "$DASHBOARD_NAME" '.dashboards[] | select(.name == $dashboard_name) | @json')

if [ -z "$matching_dashboard" ]; then
  echo "❌ Dashboard '$DASHBOARD_NAME' not found!"
  echo "Available dashboards (showing first 10):"
  echo "$dashboards_response" | jq -r '.dashboards[0:10][]?.name' 2>/dev/null || echo "No dashboards found or API response format unexpected"
  exit 1
fi

dashboard_id=$(echo "$matching_dashboard" | jq -r '.id')
dashboard_name=$(echo "$matching_dashboard" | jq -r '.name')
echo "✅ Found dashboard: $dashboard_name (ID: $dashboard_id)"

#--- get full dashboard details --------------------------------------------
echo "📥 Getting full dashboard details..."
dashboard_response=$(curl -sS -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  "$API_ENDPOINT/v3/dashboards/$dashboard_id")

if [ $? -ne 0 ]; then
  echo "❌ Failed to get dashboard details"
  exit 1
fi

#--- save raw dashboard for reference --------------------------------------
echo "💾 Saving raw dashboard JSON..."
mkdir -p output
echo "$dashboard_response" | jq . > "output/source_dashboard_raw.json"
echo "✅ Saved raw dashboard to: output/source_dashboard_raw.json"

#--- extract template content (removing unnecessary fields) ---------------
echo "🔧 Extracting template content..."

# Fields to remove from the dashboard template
fields_to_remove=("id" "createdOn" "modifiedOn" "version" "username" "customerId" "publicToken" "permissions" "favorite" "teamId" "userId" "lastAccessedOnByCurrentUser")

# Convert bash array to JSON array for jq
fields_json=$(printf '%s\n' "${fields_to_remove[@]}" | jq -R . | jq -s .)

# Extract dashboard content and remove unwanted fields, keeping API-ready format
dashboard_data=$(echo "$dashboard_response" | jq --argjson fields "$fields_json" '.dashboard | with_entries(select(.key as $k | $fields | index($k) | not))')
template_content=$(echo "{\"dashboard\": $dashboard_data}")

if [ -z "$template_content" ] || [ "$template_content" = "null" ]; then
  echo "❌ Failed to extract template content"
  exit 1
fi

#--- save template ---------------------------------------------------------
panel_count=$(echo "$template_content" | jq '.dashboard.panels | length // 0')
echo "📝 Template extracted with $panel_count panels"

# Create a safe filename from dashboard name
safe_filename=$(echo "$DASHBOARD_NAME" | sed 's/[^a-zA-Z0-9]/_/g' | tr '[:upper:]' '[:lower:]')
template_filename="template/dashboard_template_${safe_filename}.json.j2"

echo "💾 Saving template to: $template_filename"
echo "$template_content" | jq . > "$template_filename"

echo "✅ Dashboard template successfully created!"
echo "📁 Template saved as: $template_filename"
echo "📁 Raw JSON saved as: output/source_dashboard_raw.json"
