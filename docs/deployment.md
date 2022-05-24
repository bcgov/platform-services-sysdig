## Sysdig Agent Deployment
Currently Sysdig Agents are deployed with Cluster Config Management (CCM). To manually deploy Sysdig to a cluster, see the following requirements and steps.

Deploying Sysdig requires the following k8s components:

- project/namespace with additional privileges
- serviceaccount
- daemonset
- configMap
- secret

*note: disabling agents on lab:storage nodes to get the most coverage (also not enabled in production clusters at the moment)*

```bash
# create namespace
oc new-project openshift-bcgov-sysdig-agent --description='BC Gov DevOps Platform Sysdig Monitoring Platform'
oc project openshift-bcgov-sysdig-agent
# create SA and grant rolebindings
oc create serviceaccount sysdig-agent
oc adm policy add-scc-to-user privileged -z sysdig-agent
oc adm policy add-cluster-role-to-user cluster-reader -z sysdig-agent
# create sysdig secret
oc create secret generic sysdig-agent --from-literal=access-key=<your_sysdig_access_key>
# currently using docker image from account called bcdevopscluster, will switch to artifactory when ready:
oc create secret docker-registry bcgov-docker-hub --docker-server=docker.io --docker-username=bcdevopscluster --docker-password=<docker_password> --docker-email=unused
oc secrets link default bcgov-docker-hub --for=pull

# Label all cluster nodes so daemonset would find them 
oc label node --all "sysdig-agent=true"

# Step 1: copy over the openshift manifests for Sysdig from CCM repo: ds-sysdig-agent.yaml.j2 and cm-sysdig-agent.yaml.j2
# Step 2: convert the jinja2 template into yaml openshift template manifest
# Step 3: create env var for lab and production deployment: prod.env and lab.env
# Step 4: create the configmap and daemonset
echo "--- Lab cluster - skipping storage region"
for region in master infra app; do
oc apply -f openshift/cm-sysdig-agent-${region}.yaml
oc process -f openshift/ds-sysdig-agent-template.yaml --param-file=openshift/lab.env -o yaml | oc apply -f -
done

echo "--- Prod cluster - Adjust default limits and requests"
for region in master infra storage app; do
oc apply -f openshift/cm-sysdig-agent-${region}.yaml
oc process -f openshift/ds-sysdig-agent-template.yaml --param-file=openshift/prod.env -o yaml | oc apply -f -
done
```

Once this is complete, update the `openshift-bcgov-sysdig-agent` project to allow for agents to be deployed across the infra, master, and storage nodes as well.

- Edit the namespace:

``` bash
oc edit namespace openshift-bcgov-sysdig-agent
```

- Add the following line within the annotation

``` bash
    openshift.io/node-selector: ""
```
