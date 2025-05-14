# Sysdig Metrics Management Script

This **Sysdig Metrics Management** script makes it easy to list, disable, and enable custom metrics in Sysdig via the Sysdig REST API. It offers a simple command-line interface for toggling which metrics should be collected and billed.

---

## Features

- **Token Handling**  
  - Automatically retrieves your Sysdig API token from `~/.sysdig_metrics_token`.  
  - Prompts for your token only if one isn’t found.

- **List Disabled Metrics**  
  - Displays all metrics that are currently disabled.

- **Disable or Enable Metrics**  
  - Turn off (disable) or turn on (enable) individual metrics by specifying `<jobName>` and `<metricName>`.  
  - Bulk toggle metrics from a two-column file (`metricName` and `jobName`).

---

## Requirements

1. **Bash** or another compatible shell.  
2. **cURL** for making HTTP requests.  
3. **jq** for parsing JSON responses.  
4. A **Sysdig API token** with permissions to manage disabled metrics.

---

## Installation

1. Save the script to a file named `sysdig-metrics.sh` (or any name you prefer).  
2. Make it executable:  
   ```bash
   chmod +x sysdig-metrics.sh

## Usage

### List disabled metrics
```bash
bash sysdig-metrics.sh -l
```

### Disable or enable a single metric

```bash
# Disable one metric:
bash sysdig-metrics.sh -d <jobName> <metricName>

# Enable one metric:
bash sysdig-metrics.sh -e <jobName> <metricName>
```

### Bulk disable or enable from a file
Prepare a text file (e.g., current-disable-metrics.txt) where each line has:

```bash
metricName jobName

```

Example:
```
container_spec_cpu_period   k8s-cadvisor-default
kubelet_runtime_operations_total   k8s-pods
```

Then Run:

```bash
bash sysdig-metrics.sh -d metrics.txt  # disable all listed
bash sysdig-metrics.sh -e metrics.txt  # enable all listed

```


## Example
#### Disable a single metric:

```
bash sysdig-metrics.sh -d k8s-pods http_roundtrip_duration_seconds_bucket

```

#### Enable a single metric:

```
bash sysdig-metrics.sh -e k8s-pods http_roundtrip_duration_seconds_bucket

```

#### Enble a buck metcis:
```
bash sysdig-metrics.sh -d current-disabled-metrics.txt

```

#### Disable a bulk of metrics:
```
bash sysdig-metrics.sh -e current-disabled-metrics.txt

```

##### List of current disabled metrcis:
```
bash sysdig-metrics.sh -l
```

## Trouble shooting
* No token found
If you see a prompt for a token, ensure your ~/.sysdig_metrics_token file exists or enter a valid API token.

* Missing dependencies
Verify curl and jq are installed and in your PATH.

* Permission errors
Confirm the script is executable (chmod +x sysdig-metrics.sh) and that your API token has the necessary privileges.

* Invalid request or 400/403 errors
Check that you provided a valid <jobName> and <metricName>. Use -l to list currently disabled metrics and verify syntax.

## TODO:

Some metrics in k8s-default was disabled by my mistake and unable to fix from my side, Talk to Dusin to figure out how to remove those. No bad effect so far.
## Contributing

If you’d like to contribute:

1. Report any issues or feature requests.  
2. Fork the repository, make your changes, and submit a pull request.

---

## License

This script is available under the [MIT License](https://opensource.org/licenses/MIT). Feel free to modify it to suit your needs.
