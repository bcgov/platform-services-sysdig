#!/bin/bash
set -e
set -o pipefail

# --- Configuration ---
# The Sysdig team you want to create the dashboard for.
TEAM_NAME="fc7a67-team"

# The Kubernetes namespace to scope the dashboard to. Defaults to the team name.
TARGET_NAMESPACE="${TEAM_NAME}"

# Path to the dashboard template file. Assumes script is run from the project root.
DASHBOARD_TEMPLATE_PATH="./template/dashboard-resources-approve.json.j2"

# The API endpoint for your Sysdig region.
API_ENDPOINT="https://app.sysdigcloud.com"

# --- Check for dependencies ---
if ! command -v jq &> /dev/null; then
    echo "Error: jq could not be found. Please install jq to run this script."
    exit 1
fi

if ! command -v curl &> /dev/null; then
    echo "Error: curl could not be found. Please install curl to run this script."
    exit 1
fi

# --- Ensure TOKEN is set ---
if [ -z "$TOKEN" ]; then
  if [ -f ~/.sysdig_metrics_token ]; then
    TOKEN=$(<~/.sysdig_metrics_token)
    echo "Using token from ~/.sysdig_metrics_token"
  else
    read -sp "API token not found. Please enter your Sysdig API token: " TOKEN
    echo # for newline
    echo "$TOKEN" > ~/.sysdig_metrics_token
    chmod 600 ~/.sysdig_metrics_token
  fi
fi

# --- Main Script ---

# 1. Get Team ID from Team Name
echo "Fetching Team ID for team: '$TEAM_NAME'..."
# The API's 'name' filter is a 'contains' search, so we fetch all matches
# and then use jq for an exact match.
TEAM_DATA=$(curl -s -X GET \
  -H "Authorization: Bearer $TOKEN" \
  "$API_ENDPOINT/platform/v1/teams?filter=name:$TEAM_NAME")

# Extract the team ID using jq for an exact name match.
# The '-r' flag gives the raw string without quotes.
TEAM_ID=$(echo "$TEAM_DATA" | jq -r ".data[] | select(.name==\"$TEAM_NAME\") | .id")

# Check if we found exactly one team.
if [ -z "$TEAM_ID" ]; then
    echo "Error: Could not find any team with the exact name '$TEAM_NAME'."
    echo "API returned the following potential matches:"
    echo "$TEAM_DATA" | jq '.data[] | .name'
    exit 1
elif [ "$(echo "$TEAM_ID" | wc -l)" -ne 1 ]; then
    echo "Error: Found multiple teams with the exact name '$TEAM_NAME'. Please use a unique team name."
    echo "Found IDs:"
    echo "$TEAM_ID"
    exit 1
fi

echo "Found unique Team ID: $TEAM_ID"

# 2. Prepare Dashboard JSON payload from the template
echo "Preparing dashboard payload using template: '$DASHBOARD_TEMPLATE_PATH'"
if [ ! -f "$DASHBOARD_TEMPLATE_PATH" ]; then
    echo "Error: Dashboard template not found at '$DASHBOARD_TEMPLATE_PATH'"
    exit 1
fi

# Substitute the placeholders in the template using sed.
DASHBOARD_JSON=$(cat "$DASHBOARD_TEMPLATE_PATH" | \
    sed "s/__TEAM_ID__/$TEAM_ID/g" | \
    sed "s/__TARGET_NAMESPACE__/$TARGET_NAMESPACE/g")

# --- DEBUG: Print the final JSON payload to be sent ---
# echo "--- BEGIN JSON PAYLOAD ---"
# echo "$DASHBOARD_JSON"
# echo "--- END JSON PAYLOAD ---"

# 3. Create the Dashboard via API call
echo "Sending request to create dashboard for team '$TEAM_NAME'..."
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d "$DASHBOARD_JSON" \
  "$API_ENDPOINT/api/v3/dashboards")

# Extract HTTP status code and response body
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

# 4. Check the API response
if [ "$HTTP_CODE" -eq 201 ]; then
    echo "Successfully created dashboard."
    echo "Response:"
    echo "$BODY" | jq .
else
    echo "Error: Failed to create dashboard. Sysdig API responded with HTTP status $HTTP_CODE."
    echo "Response:"
    # Try to pretty-print with jq if it's JSON, otherwise just print the raw body
    if echo "$BODY" | jq . > /dev/null 2>&1; then
        echo "$BODY" | jq .
    else
        echo "$BODY"
    fi
    exit 1
fi
