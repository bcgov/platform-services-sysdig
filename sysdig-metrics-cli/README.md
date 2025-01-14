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
  - Quickly turn off (disable) or turn on (enable) any metrics you no longer need or want to resume tracking.

- **Use of `current-disabled-metrics.txt`**  
  - A handy way to keep track of which metrics you currently have disabled or plan to disable.  
  - You can maintain a list of metric names (one per line) in this file.  
  - When you want to disable or enable them again, simply copy those metric names and pass them to the script or use the command syntax below.

---

## Requirements

1. **Bash** or another compatible shell.  
2. **cURL** for making HTTP requests.  
3. **jq** for parsing JSON responses.  
4. A **Sysdig API token** with the necessary permissions to manage disabled metrics.

---

## Installation

1. Save the script to a file named `sysdig-metrics` (or any name you prefer).  
2. Make it executable (for example, by running `chmod +x sysdig-metrics`).  
3. (Optional) Move or symlink it to a directory in your `PATH` to run it globally (e.g., `/usr/local/bin`).

---

## Usage

- **Listing disabled metrics (`-l`)**  
  - Retrieves a list of metrics that are currently disabled in Sysdig.
  - example `bash sysdig-metrics.sh -l`

- **Disabling metrics (`-d`)**  
  - Specify one or more metric names to disable.
  - Example: `bash sysdig-metrics.sh -d kubelet_runtime_operations_total`

- **Enabling metrics (`-e`)**  
  - Specify one or more metric names that you previously disabled to enable them again.
  - Example: `bash sysdig-metrics.sh -e kubelet_runtime_operations_total`
- **Using `current-disabled-metrics.txt`**  
  - Create or maintain this text file with a list of metric names (one per line) to keep track of your current disabled metrics or the metrics you intend to disable.  
  - **Correct Usage**:  
    ```bash
    bash sysdig-metrics.sh -d $(cat current-disabled-metrics.txt)
    ```
    This command disables all metrics listed in `current-disabled-metrics.txt`.

---

## Examples

1. **Disable a single metric**  
   - Disable a single metric by passing its name:
     ```bash
     bash sysdig-metrics.sh -d custom_metric_a
     ```

2. **Disable multiple metrics**  
   - Disable multiple metrics by listing them (separated by spaces):
     ```bash
     bash sysdig-metrics.sh -d custom_metric_a custom_metric_b custom_metric_c
     ```

3. **Use `current-disabled-metrics.txt`**  
   - Place all the metrics you want to disable (one per line) into `current-disabled-metrics.txt`. Then run:
     ```bash
     bash sysdig-metrics.sh -d $(cat current-disabled-metrics.txt)
     ```
   - This way, you don’t need to copy/paste each metric name manually every time.

4. **Enable a metric**  
   - Re-enable a metric that was previously disabled:
     ```bash
     bash sysdig-metrics.sh -e custom_metric_a
     ```

5. **List current disabled metrics**  
   - Run with the list option to see which metrics are disabled:
     ```bash
     bash sysdig-metrics.sh -l
     ```

---

## Troubleshooting

1. **No token found**  
   - The script will prompt you for a token if one isn’t located in `~/.sysdig_metrics_token`.

2. **Missing dependencies**  
   - Make sure both cURL and jq are installed and available in your `PATH`.

3. **Permission errors**  
   - Ensure the script has execute permissions (e.g., `chmod +x sysdig-metrics`).
   - Verify your API token has the required privileges in Sysdig.

4. **Unauthorized / 403**  
   - Confirm your API token is valid and has permissions to modify disabled metrics.

---

## Contributing

If you’d like to contribute:

1. Report any issues or feature requests.  
2. Fork the repository, make your changes, and submit a pull request.

---

## License

This script is available under the [MIT License](https://opensource.org/licenses/MIT). Feel free to modify it to suit your needs.
