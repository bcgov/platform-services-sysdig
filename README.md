![Lifecycle:Maturing](https://img.shields.io/badge/Lifecycle-Maturing-007EC6)

# Sysdig Monitor and Secure

### Purpose
Sysdig is the centralized monitoring tool to support both Monitoring Operations teams and Application teams across the BCGov OpenShift platform. This solution will remove the dependency on "in cluster" monitoring tools and will scale well with additional clusters and cloud workloads. 

### What's where:

Here in this repo, we have:
- [Sysdig Team Operator](./operator/readme.md) that manages Users, Teams and Dashboard Templates in the Sysdig Monitor and Secure platform.
- [Additional scripts](./scripts/readme.md) to manage the existing sysdig resources that the operator creates
- [Sysdig metrics scripts](./sysdig-metrics-cli/README.md) that controls what metrics to enable/disable on sysdig cloud

The following are located outside of this repo:
- [Developer User Guide](https://github.com/bcgov/platform-developer-docs/blob/main/src/docs/app-monitoring/sysdig-monitor-onboarding.md)
- [Sysdig Installation](https://github.com/bcgov-c/platform-gitops-sysdig)
- [All Operation/Maintenance related topics](https://github.com/bcgov-c/platform-gitops-sysdig/blob/main/docs/maintenance.md)
- [Gitops of this Operator is part of CCM](https://github.com/bcgov-c/platform-gitops-gen/tree/master/roles/sysdig_teams_operator)

### Help / Contact
See `#devops-sysdig` or `#devops-how-to` channels for assistance in [Rocket.Chat](https://chat.developer.gov.bc.ca/).
