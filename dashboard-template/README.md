# Dashboard Template Tools

This directory contains scripts for pulling dashboard templates from Sysdig and creating new dashboards from templates.

## Prerequisites

- Sysdig API token (found in https://app.sysdigcloud.com/#/settings/user under "Sysdig Monitor API Token")
- `jq` and `curl` installed

## Scripts

### 1. Pull Dashboard Template (`pull_dashboard.sh`) (Mainly for Admin use)

Pulls a dashboard from Sysdig and exports it as a reusable template.

**Usage:**
```bash
./pull_dashboard.sh <api_token> "<dashboard_name>"
```

**Example:**
```bash
./pull_dashboard.sh "your-api-token" "Resource Allocation Dashboard"
```

**What it does:**
1. Uses the provided Sysdig API token
2. Uses the Dashboard Listing API to find all accessible dashboards
3. Locates the specified dashboard by name
4. Downloads the full dashboard configuration using the Get Dashboard by ID API
5. Removes team-specific and user-specific fields to create a reusable template
6. Saves the template as `template/dashboard_template_<safe_name>.json.j2`
7. Saves the raw JSON as `output/source_dashboard_raw.json` for reference

**To find your API token:**
1. Go to https://app.sysdigcloud.com/#/settings/user
2. Look under "Sysdig Monitor API Token"

**To find team IDs:**
```bash
# Get formatted list of teams with IDs
TOKEN='YOUR_API_TOKEN'
printf "%-30s %s\n" "Team Name" "Team ID"
printf "%-30s %s\n" "=========" "======="

curl -s -H "Authorization: Bearer $TOKEN" "https://app.sysdigcloud.com/api/v3/teams" | \
  jq -r '.data[] | "\(.name)\t\(.id)"' | \
  while IFS=$'\t' read -r name id; do
    printf "%-30s %s\n" "$name" "$id"
  done
```

### 2. Create Dashboard from Template (`create_dashboard_from_template.sh`)

Creates a new dashboard in a specific team using a template file.

**Usage:**
```bash
./create_dashboard_from_template.sh <api_token> <template_file> <target_team_id>
```

**Example:**
```bash
./create_dashboard_from_template.sh "your-api-token" template/dashboard_template_resources_quota_approve_dashboard.json.j2 50396
```

**What it does:**
1. Uses the provided API token
2. Loads the template file and injects team-specific information
3. Creates a new dashboard using the Create Dashboard API
4. Saves the created dashboard details for reference


## Conclusion

1. **Find your API token**: Go to https://app.sysdigcloud.com/#/settings/user → "Sysdig Monitor API Token"
2. **List available teams**: Run the curl command shown in "To find team IDs" section below
3. **Pull template**: `./pull_dashboard.sh <api_token> "Source Dashboard"`
4. **Create dashboard**: `./create_dashboard_from_template.sh <api_token> template/file.json.j2 <team_id>`
