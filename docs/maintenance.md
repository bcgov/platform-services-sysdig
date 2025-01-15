# Sysdig Maintenance

## Sysdig Installation
Currently Sysdig components are deployed with Cluster Config Management (CCM). To manually deploy Sysdig to a cluster, follow the steps in the [CCM repo](https://github.com/bcgov-c/platform-gitops-gen/blob/master/roles/sysdig_agent/readme.md).

After that's done, you'll need to setup the authentication. Sysdig is configured with OpenID Connect to leverage the BCGov Keycloak SSO instance for authentication.

### OpenID Connect Configuration
Keycloak and Sysdig were configured manually for the OIDC integration. The following screenshots were used to configure each component (they are a bit outdated, make sure to check the actual value when setting up):

![](assets/sysdig_oidc_kcsso_01.png)
![](assets/sysdig_oidc_sysdig_01.png)

- Keycloak Realm: the `platform-services` realm is used for the integration
- Client ID: could be found from keycloak's client ID
- Client Secret: is from the credential tab from keycloak
- The Client Org: is set to be `BCDevOps`, which is used when logging in with OpenID

## Testing Sysdig Changes
If you need to test out an upgrade, or simply a configuration change:
- disable auto-sync from ArgoCD for a lab clusters (klab/clab/klab2)
- follow the [CCM repo](https://github.com/bcgov-c/platform-gitops-gen/blob/master/roles/sysdig_agent/readme.md) to make the changes in the values.yaml and populate the change to the generated manifests
- apply the change in the lab cluster instance for testing

## Troubleshooting
Here are some helpful commands:

```bash
oc project openshift-bcgov-sysdig-agent

mkdir zip

oc get daemonset sysdig-agent -o yaml > zip/sysdig-agent.daemonset.yaml
oc get configmap sysdig-agent -o yaml > zip/sysdig-agent.configmap.yaml
oc get daemonset sysdig-agent-node-analyzer -o yaml > zip/sysdig-agent-node-analyzer.daemonset.yaml
oc get deployment sysdig-agent-clustershield -o yaml > zip/sysdig-agent-clustershield.deployment.yaml

# get all logs and sysdig agent gather script:
./get-pod-logs.sh
./agent-gather.sh -d ./zip

# copy the corresponding values.yaml to /zip

# zip:
zip -r zip.zip zip

# set upload URL: 
SYSDIG_UPLOAD_URL="given-by-sysdig-support"

# upload:
curl -s -S -X PUT --url "$SYSDIG_UPLOAD_URL" -H "Content-Disposition: attachment; filename=zip.zip" -T zip.zip
```

Sometimes you might also need to:
- enable a more verbose logging level from the helm chart values.yaml
- run a node capture when troubleshooting an issue happening to some specific node/sysdig agent pods: https://docs.sysdig.com/en/docs/sysdig-secure/threats/investigate/captures/


## Getting Support
If you encounter an issue from Sysdig that you cannot resolve, reach out to the Sysdig support via a ticket at https://cx.sysdig.com/s/cases

It's also helpful to connect with our Customer Solutions Engineer, Dustin Krysak, on RocketChat or dustin.krysak@sysdig.com.

## Sysdig Subscription
There is a limit on how many nodes we can install Sysdig onto across all OpenShift clusters.

Sysdig Monitor and Secure create two daemonsets in each cluster, which requires one license per node. To find out how many licenses are available, check from Sysdig Cloud [subscription details](https://app.sysdigcloud.com/#/settings/subscription).

The license also entitle to a certain amount of timeseries (metrics), that's why we have disabled collection for metrics not used by teams.
