# sysdig-operator
sysdig-operator manages the configuration of user access to Sysdig based on information from the Registry.

Create one `SysdigTeam` per `-tools` namespace. Edit its user list to add or remove members; the operator sends an invitation for new users. KLAB rejects a second resource in the same namespace. Update the original, or delete it before creating a replacement.

## Description

## Getting Started

### Prerequisites
- go version v1.24.0+
- docker version 17.03+.
- kubectl version v1.11.3+.

### Test changes locally
After making code changes, verify that the operator can still be built without errors.  This is best done on your local machine, rather than committing the code, pushing it to the code repo, and then having the OpenShift Pipeline do a build.

To test the build, change to the base directory of this repo and run: `make build`

If there are any errors, investigate and fix them.

Normal output from running 'make build' looks something like this:
```
/Users/username/repos/bcgov/platform-services-sysdig/sysdig-operator/bin/controller-gen-v0.14.0 rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
/Users/username/repos/bcgov/platform-services-sysdig/sysdig-operator/bin/controller-gen-v0.14.0 object:headerFile="hack/boilerplate.go.txt" paths="./..."
go fmt ./...
go vet ./...
go build -o bin/manager cmd/main.go
```

### Run the operator locally with `billy-test.yaml`

Copy `sysdigteam-sample.yml` to `billy-test.yaml`, then set the test namespace, resource name, users, and roles. The operator may remove team members who are not listed.

1. In [KLAB Argo CD](https://gitops.apps.klab.devops.gov.bc.ca), turn off auto-sync for `cluster-apps` and `sysdig-teams-operator`. Record the current replica count, then scale the [sysdig-operator-go deployment](https://console.apps.klab.devops.gov.bc.ca/k8s/ns/openshift-bcgov-sysdig-agent/deployments/sysdig-operator-go) to **0**. Wait for **0/0** ready.
2. Check that you are using the right cluster and review the resources in the test namespace:

   ```sh
   oc config current-context
   oc get sysdig-team -n c9ba43-tools
   oc diff -f billy-test.yaml
   ```

3. In this directory, start the operator:

   ```sh
   source .env
   go run ./cmd/main.go --watch-namespace=c9ba43-tools --health-probe-bind-address=0
   ```

4. In another terminal, apply the test resource:

   ```sh
   oc apply -f billy-test.yaml
   oc get sysdig-team c9ba43-sysdigteam -n c9ba43-tools -o yaml
   ```

5. In Sysdig, check that `c9ba43-team-secure-zone` exists and `c9ba43-team-secure` selects it. Check what a non-admin team member can see before deployment; selecting a zone alone does not limit access.
6. Press Ctrl+C to stop the local operator. Restore the deployment's previous replica count and the previous auto-sync settings.

## Automated Builds
Builds are started automatically when changes are pushed to the repo or when a release is created.  A webhook in the repo makes a call to an OpenShift Pipeline called `operator-build` in the Silver `gitops-tools` namespace.

* Upon any push to the repo, the pipeline will build the image with the `latest` tag.
* Upon the creation of a release in the repo, the pipeline will build the image with the tag matching the release version, such as `v1.2.3`.

## Testing Images in OpenShift
Test the new image in the same way that we test CCM changes:
* Log in to the KLAB CCM instance of ArgoCD: https://gitops.apps.klab.devops.gov.bc.ca
* Edit the `cluster-apps` Application and disable auto-sync.
* Edit the `sysdig-teams-operator` Application and disable auto-sync.
* Update the `sysdig-operator-go` Deployment in the openshift-bcgov-sysdig-agent namespace, changing the image tag to `latest`.
* Monitor the logs of the `manager` container in the new pod.
* When done testing, re-enable auto-sync in `cluster-apps`, including the prune and self-heal options.

## Prepare a Release
After successfully testing the new image:
* Create a pull request from your development branch into `main`
* Merge the pull request and delete the development branch
* Create a Release
    * Click the 'Releases' link in the GitHub UI
    * Click the 'Draft a new release' button
    * Click the 'Tag: Select tag' button
    * In the input field 'Search or create a new tag', enter the new tag, such as `v1.2.3`
    * After entering the new tag, the tag list is replaced with a link reading `Create new tag: v1.2.3 on publich` - click that link
    * Enter a title and description for the release
    * Click 'Publish release'

The creation of the release will start a PipelineRun in the Silver `gitops-tools` namespace.  Verify that the build completes successfully.

## Prepare Update
Similar to the testing of a `latest` image, test the new versioned image.  After successful testing, prepare a CCM PR by updating the image tag in `roles/sysdig_teams_operator/defaults/main.yaml`.
