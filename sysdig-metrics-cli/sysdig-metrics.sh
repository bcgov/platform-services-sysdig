#!/bin/bash

API_ENDPOINT="https://api.us1.sysdig.com"

# Prompt for token if not provided and export it for future use
if [ -z "$TOKEN" ]; then
  if [ -f ~/.sysdig_metrics_token ]; then
    TOKEN=$(cat ~/.sysdig_metrics_token)
  else
    read -p "API token not found. Please enter your API token: " TOKEN
    echo "$TOKEN" > ~/.sysdig_metrics_token
  fi
fi

export TOKEN
echo "Using token: $TOKEN"

function list_metrics() {
  response=$(curl --location --request GET "$API_ENDPOINT/monitor/prometheus-jobs/v1/disabled-metrics" \
  --header "Authorization: Bearer $TOKEN")
  echo "Disabled metrics:"
  echo "$response" | jq -r '.data[] | .metrics[] | .metricName'
}

function disable_metrics() {
  local is_disabled=$1
  shift
  local metrics=($@)
  local metrics_data=""

  for metric in "${metrics[@]}"; do
    metrics_data+="{
      \"metricName\": \"$metric\",\"isDisabled\": $is_disabled
    },"
  done

  # Replace the line where `sed` is used with the correct pattern to remove the last comma
  metrics_data=$(echo "$metrics_data" | sed 's/,$//')
  echo "what is this: $metrics_data"
  response=$(curl --location --request POST "$API_ENDPOINT/monitor/prometheus-jobs/v1/disabled-metrics" \
    --header "Authorization: Bearer $TOKEN" \
    --header 'Content-Type: application/json' \
    --header 'X-sysdig-public-notation: true' \
    --data-raw "{
       \"data\": [
          {
             \"jobName\": \"k8s-pods\",
             \"metrics\": [
                $metrics_data
             ]
          }
       ]
    }")

  # Use jq to parse the response and check if there are any errors
  errors=$(echo "$response" | jq -r '.errors | length')
  if [ "$errors" -eq 0 ]; then
    echo "Success: Metrics updated successfully."
  else
    echo "Error: Failed to update metrics. Response: $response"
  fi
}

# CLI logic
if [ "$1" == "-l" ]; then
  list_metrics
elif [ "$1" == "-d" ]; then
  shift
  if [ $# -eq 0 ]; then
    echo "Error: No metrics provided to disable."
    echo "Usage: sysdig-metrics -d metric1 metric2 ... (disable metrics)"
    exit 1
  fi
  disable_metrics true "$@"
elif [ "$1" == "-e" ]; then
  shift
  if [ $# -eq 0 ]; then
    echo "Error: No metrics provided to enable."
    echo "Usage: sysdig-metrics -e metric1 metric2 ... (enable metrics)"
    exit 1
  fi
  disable_metrics false "$@"
else
  echo "Error: Invalid option provided."
  echo "Usage: sysdig-metrics -l (list metrics)"
  echo "       sysdig-metrics -d metric1 metric2 ... (disable metrics)"
  echo "       sysdig-metrics -e metric1 metric2 ... (enable metrics)"
  exit 1
fi
