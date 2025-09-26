# sysdig-operator
sysdig-operator manages the configuration of user access to Sysdig based on information from the Registry.

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

You can run the manager locally if you want.  Log in to KLAB and then start the operator:
```
go run bin/manager
```

When you are ready to test your changes in OpenShift, commit your changes to a branch in the code repo.

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

