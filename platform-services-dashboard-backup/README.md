# Platform Services Dashboard Backup

Backs up the current user's Sysdig dashboards as JSON and restores them by dashboard ID.

## Requirements

- `curl` and `jq`
- A Sysdig Monitor API token in `~/.sysdig_metrics_token` or the `TOKEN` environment variable

## Backup

From the repository root:

```bash
./platform-services-dashboard-backup/backup_dashboards.sh
```

Dashboard JSON files are stored in `platform-services-dashboard-backup/dashboards/`.

## Restore

Preview changes without updating Sysdig:

```bash
./platform-services-dashboard-backup/restore-dashboard.sh \
  platform-services-dashboard-backup/dashboards/Cluster-Capacity.json
```

Apply the JSON to Sysdig:

```bash
./platform-services-dashboard-backup/restore-dashboard.sh \
  platform-services-dashboard-backup/dashboards/Cluster-Capacity.json \
  --apply
```

An existing dashboard is updated in place. If it was deleted, a new dashboard is created and its new ID is written back to the JSON file. Before an update, the current remote version is saved under `restore-backups/`.

## GitHub Actions

The `Backup Sysdig dashboards` workflow runs daily and can also be started manually. Add the dashboard owner's API token as the repository Actions secret `SYSDIG_METRICS_TOKEN`.
