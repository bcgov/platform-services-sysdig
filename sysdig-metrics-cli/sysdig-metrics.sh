#!/usr/bin/env bash
set -eo pipefail

API_ENDPOINT="https://api.us1.sysdig.com"

#--- ensure TOKEN is set ---------------------------------------------------
if [ -z "$TOKEN" ]; then
  if [ -f ~/.sysdig_metrics_token ]; then
    TOKEN=$(<~/.sysdig_metrics_token)
  else
    read -p "API token not found. Please enter your API token: " TOKEN
    echo "$TOKEN" > ~/.sysdig_metrics_token
  fi
fi
export TOKEN

#--- list currently disabled metrics ---------------------------------------
function list_metrics() {
  curl -sS -H "Authorization: Bearer $TOKEN" \
    "$API_ENDPOINT/monitor/prometheus-jobs/v1/disabled-metrics" \
  | jq -r '.data[]? | "Job: \(.jobName)", (.metrics[]? | "  - \(.metricName)")'
}

#--- disable or enable single metric --------------------------------------
# $1 = true|false, $2 = jobName, $3 = metricName
function toggle_single_metric() {
  local is_disabled=$1
  local job=$2
  local metric=$3

  payload="{\"data\":[{\"jobName\":\"$job\",\"metrics\":[{\"metricName\":\"$metric\",\"isDisabled\":$is_disabled}]}]}"

  echo "Payload to be sent:"
  echo "$payload" | jq .

  response=$(curl -sS -X POST \
    "$API_ENDPOINT/monitor/prometheus-jobs/v1/disabled-metrics" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -H "X-sysdig-public-notation: true" \
    --data-raw "$payload")

  local errors
  errors=$(echo "$response" | jq -r '.errors | length // 0')
  if [ "$errors" -eq 0 ]; then
    echo "✅ Success: Metric '$metric' on job '$job' updated."
  else
    echo "❌ Failed to update. Response was:"
    echo "$response" | jq .
    exit 1
  fi
}

#--- disable or enable from file -------------------------------------------
function toggle_from_file() {
  local is_disabled=$1
  local file=$2

  if [ ! -f "$file" ]; then
    echo "Error: file '$file' not found."
    exit 1
  fi

  declare -A groups
  while read -r metric job _; do
    [[ -z "$metric" || "${metric:0:1}" == "#" ]] && continue
    groups["$job"]+="$metric "
  done < "$file"

  local payload
  payload='{"data":['
  for jobName in "${!groups[@]}"; do
    payload+="{\"jobName\":\"$jobName\",\"metrics\":["
    for m in ${groups[$jobName]}; do
      payload+="{\"metricName\":\"$m\",\"isDisabled\":$is_disabled},"
    done
    payload=${payload%,}
    payload+=']},'
  done
  payload=${payload%,}
  payload+=']}'

  echo "Payload to be sent:"
  echo "$payload" | jq .

  response=$(curl -sS -X POST \
    "$API_ENDPOINT/monitor/prometheus-jobs/v1/disabled-metrics" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -H "X-sysdig-public-notation: true" \
    --data-raw "$payload")

  local errors
  errors=$(echo "$response" | jq -r '.errors | length // 0')
  if [ "$errors" -eq 0 ]; then
    echo "✅ Success: Metrics updated from file '$file'."
  else
    echo "❌ Failed to update. Response was:"
    echo "$response" | jq .
    exit 1
  fi
}

#--- CLI dispatch -----------------------------------------------------------
case "$1" in
  -l)
    list_metrics
    ;;
  -d|-e)
    # detect disable vs enable flag
    is_flag_disable=false
    if [ "$1" = "-d" ]; then
      is_flag_disable=true
    fi
    shift

    if [ $# -eq 2 ]; then
      # direct metric mode: jobName metricName
      toggle_single_metric "$is_flag_disable" "$1" "$2"
    elif [ $# -eq 1 ]; then
      # file mode: single arg is file
      toggle_from_file "$is_flag_disable" "$1"
    else
      echo "Usage: $0 -d|-e <jobName> <metricName>" >&2
      echo "       $0 -d|-e <metrics-file.txt>" >&2
      exit 1
    fi
    ;;
  *)
    cat <<EOF
Usage:
  $0 -l
      List currently disabled metrics.

  # Disable or enable a single metric:
  $0 -d <jobName> <metricName>
  $0 -e <jobName> <metricName>

  # Or disable/enable in bulk from a two-column file:
  $0 -d <metrics-file.txt>
  $0 -e <metrics-file.txt>
EOF
    exit 1
    ;;
esac
