/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	helpers "github.com/bcgov/platform-services-sysdig/sysdig-operator/internal/helper"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"fmt"
	"os"
	"regexp"
	"strings"

	api "github.com/bcgov/platform-services-sysdig/sysdig-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"

	monitoringv1alpha1 "github.com/bcgov/platform-services-sysdig/sysdig-operator/api/v1alpha1"
)

// SysdigTeamGoReconciler reconciles a SysdigTeamGo object
type SysdigTeamGoReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

var validRoles = map[string]struct{}{
	"ROLE_TEAM_NONE":            {},
	"ROLE_TEAM_READ":            {},
	"ROLE_TEAM_SERVICE_MANAGER": {},
	"ROLE_TEAM_STANDARD":        {},
	"ROLE_TEAM_EDIT":            {},
	"ROLE_TEAM_MANAGER":         {},
}

func (r *SysdigTeamGoReconciler) syncOneTeam(
	apiEndpoint, token, teamName, product, description string,
	namespaces []string,
) (int64, error) {
	// Fetch existing teams by name filter
	// fmt.Printf("wtf is this team %v\n", filtered)
	filtered, err := helpers.FetchTeams(apiEndpoint, token, teamName)
	fmt.Printf("Debug: what is this exists team %+v\n", filtered)
	if err != nil {
		return 0, fmt.Errorf("fetch teams for %q: %w", teamName, err)
	}

	// Find exact, case-insensitive match
	var exists *helpers.SysdigTeam
	for i := range filtered {
		if strings.EqualFold(filtered[i].Name, teamName) {
			exists = &filtered[i]
			break
		}
	}
	fmt.Printf("wtf is this exists team %+v\n", exists)
	// Create if missing
	if exists == nil {
		id, err := helpers.CreateTeam(
			apiEndpoint,
			token,
			teamName,
			description,
			product,
			namespaces,
		)
		if err != nil {
			return 0, fmt.Errorf("create %s team: %w", product, err)
		}

		r.Log.Info("Created Sysdig team", "product", product, "name", teamName, "id", id)
		return id, nil
	} else {
		// we don't update team. as we use membership api to manage access.
		r.Log.Info("Sysdig team exists, skipping create", "product", product, "name", exists.Name, "id", exists.ID)
		return exists.ID, nil
	}
}

// +kubebuilder:rbac:groups=monitoring.devops.gov.bc.ca,resources=sysdig-team-go,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.devops.gov.bc.ca,resources=sysdig-team-go/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitoring.devops.gov.bc.ca,resources=sysdig-team-go/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SysdigTeamGo object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *SysdigTeamGoReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	apiEndpoint := os.Getenv("SYSDIG_API_ENDPOINT")
	token := os.Getenv("SYSDIG_TOKEN")

	fmt.Printf("DEBUG: Reconciling SysdigTeamGo: %s\n", req.NamespacedName)

	// Step 0: set fact
	facts := helpers.SetTeamFacts(req.Namespace)
	fmt.Printf("DEBUG: Check fact Computed nsPrefix!: %+v\n", facts)
	// fmt.Printf("Computed nsPrefix: %s\n", facts.NSPrefix)

	// Step 1: Fetch the CR instance
	var sysdigTeam api.SysdigTeamGo
	if err := r.Get(ctx, req.NamespacedName, &sysdigTeam); err != nil {
		if errors.IsNotFound(err) {
			// CR not found, possibly deleted, return.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// step 1.5 verify credentials
	if apiEndpoint == "" || token == "" {
		errMsg := "Environment variables SYSDIG_API_ENDPOINT and/or SYSDIG_TOKEN are not set"
		fmt.Printf("ERROR: %s\n", errMsg)
		sysdigTeam.Status.Conditions = []api.Condition{
			{
				Type:    "SysdigCredentials",
				Status:  "False",
				Reason:  "MissingEnvVars",
				Message: errMsg,
			},
		}
		_ = r.Status().Update(ctx, &sysdigTeam)
		return ctrl.Result{}, fmt.Errorf(errMsg)
	}

	// STEP 2 varify if object is in tools namespace
	match, err := regexp.MatchString("(?i).*-tools$", req.Namespace)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("regex validation error: %v", err)
	}

	if !match {
		errMsg := "Object must be deployed in a namespace ending with '-tools'"
		fmt.Printf("ERROR: %s\n", errMsg)

		sysdigTeam.Status.Conditions = []api.Condition{
			{
				Type:    "NamespaceValidation",
				Status:  "False",
				Reason:  "InvalidNamespace",
				Message: errMsg,
			},
		}
		_ = r.Status().Update(ctx, &sysdigTeam)

		// Stop reconciliation until fixed
		return ctrl.Result{}, nil
	}

	fmt.Printf("Namespace %s passed validation.\n", req.Namespace)

	// 3) Build teamUserList from the CR’s spec
	teamUserList := make([]helpers.TeamUserRole, 0, len(sysdigTeam.Spec.Team.Users))
	for _, u := range sysdigTeam.Spec.Team.Users {
		teamUserList = append(teamUserList, helpers.TeamUserRole{
			Name: u.Name, // e.g. "billy.li@gov.bc.ca"
			Role: u.Role, // e.g. "ROLE_TEAM_EDIT"
		})
	}
	fmt.Printf("DEBUG: teamUserList from CR: %+v\n", req)
	fmt.Printf("DEBUG: raw CR Spec: %+v\n", sysdigTeam.Spec)

	// 4) Reconcile each user one by one
	var teamUsersAndRoles []helpers.TeamUserRole
	for _, tu := range teamUserList {
		// Try to fetch by email filter
		matched, err := helpers.FetchUsers(apiEndpoint, token, tu.Name)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("fetch user %q: %w", tu.Name, err)
		}

		var userID int64
		if len(matched) > 0 {
			// user already exists
			userID = matched[0].ID
			fmt.Printf("DEBUG: user %q exists as ID %d\n", tu.Name, userID)
		} else {
			// create new user
			userID, err = helpers.CreateUser(apiEndpoint, token, tu.Name, tu.Role)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("create user %q: %w", tu.Name, err)
			}
			fmt.Printf("DEBUG: created user %q with ID %d\n", tu.Name, userID)
		}

		// build the final list
		teamUsersAndRoles = append(teamUsersAndRoles, helpers.TeamUserRole{
			Name:   tu.Name,
			Role:   tu.Role,
			UserID: userID,
		})
	}

	fmt.Printf("DEBUG: final teamUsersAndRoles = %+v\n", teamUsersAndRoles)

	// // 4.5) Validate the role.... nvm, I found that sysdig will do validation too.
	// for _, m := range teamUsersAndRoles {
	//   if _, ok := validRoles[m.Role]; !ok {
	//       return ctrl.Result{}, fmt.Errorf(
	//           "invalid role %q for user %d: must be one of %v",
	//           m.Role, m.UserID, keys(validRoles),
	//       )
	//   }

	// 5) Sync monitor team
	monitorTeamID, err := r.syncOneTeam(
		apiEndpoint,
		token,
		facts.ContainerTeamName,
		"monitor",
		sysdigTeam.Spec.Team.Description,
		facts.Namespaces,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// 6) Sync secure team and get its ID
	secureTeamID, err := r.syncOneTeam(
		apiEndpoint,
		token,
		facts.ContainerSecureTeamName,
		"secure",
		sysdigTeam.Spec.Team.Description,
		facts.Namespaces,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	fmt.Printf("check ID's Monitoring: %d, Secure:%d\n", monitorTeamID, secureTeamID)
	// 7) Assign or update memberships for each user in both teams
	for _, m := range teamUsersAndRoles {
		// Monitor team membership

		fmt.Printf("Debug, what is in the loop, :%+v\n", m)

		Monitorerr := helpers.SaveMembership(
			apiEndpoint,
			token,
			monitorTeamID,
			m.UserID,
			m.Role,
		)
		if Monitorerr != nil {
			r.Log.Error(err, "SaveMembership failed for monitor team",
				"teamID", monitorTeamID, "userID", m.UserID, "role", m.Role)
		}

		// Secure team membership
		Secureerr := helpers.SaveMembership(
			apiEndpoint,
			token,
			secureTeamID,
			m.UserID,
			m.Role,
		)
		if Secureerr != nil {
			r.Log.Error(err, "SaveMembership failed for secure team",
				"teamID", secureTeamID, "userID", m.UserID, "role", m.Role)
		}

		fmt.Printf("finish user %s\n", m.Name)
	}
	// --------------------------------------------------------------------------

	// 	filteredMonTeam, _ := helpers.FetchTeams(apiEndpoint, token, facts.ContainerTeamName)
	// // teams, err := helpers.FetchTeams(apiEndpoint, token, "sysdigteamtest-team")
	// if err != nil {
	// 	return ctrl.Result{}, fmt.Errorf("failed to fetch teams by filter: %w", err)
	// }
	// // 5) Find if the team already exist. Look for an exact (case-insensitive) name match in that small slice
	// // NOTE, this is for monitring team
	// var existingListofTeams *helpers.SysdigTeam
	// for i := range filteredMonTeam {
	// 	if strings.EqualFold(filteredMonTeam[i].Name, facts.ContainerTeamName) {
	// 		existingListofTeams = &filteredMonTeam[i]
	// 		break
	// 	}
	// }

	// fmt.Printf("wtf is this team %v\n", existingListofTeams)
	// // 6.1) If not exist, create, if exist, update
	// if existingListofTeams != nil {
	// 	// update existing
	// 	id := filteredMonTeam[0].ID
	// 	// version := teams[0].Version
	// 	// newID, _ := xhelpers.UpdateTeam(apiEndpoint, token, facts.ContainerTeamName, sysdigTeam.Spec.Team.Description, id, version, facts.Namespaces, teamUsersAndRoles)
	// 	fmt.Printf("Monitor team already exists, skipping create:%d\n", id)
	// } else {
	// 	// create new
	// 	monitorID, err := helpers.CreateTeam(
	// 		apiEndpoint,
	// 		token,
	// 		facts.ContainerTeamName,          // name
	// 		sysdigTeam.Spec.Team.Description, // description
	// 		"monitor",                        // product
	// 		facts.Namespaces,                 // namespaces → scopes built under the hood
	// 	)
	// 	if err != nil {
	// 		return ctrl.Result{}, err
	// 	}
	// 	fmt.Printf("Created team ID %d\n", monitorID)
	// }
	// // 6,2) Try to fetch the secure team by name
	// // NOTE THIs IS FOR MONITORING TEAM
	// filteredSec, err := helpers.FetchTeams(apiEndpoint, token, facts.ContainerSecureTeamName)
	// if err != nil {
	// 	return ctrl.Result{}, fmt.Errorf("fetch secure teams: %w", err)
	// }

	// // 2) Look for an exact (case‐insensitive) match
	// var secureExists *helpers.SysdigTeam
	// for i := range filteredSec {
	// 	if strings.EqualFold(filteredSec[i].Name, facts.ContainerSecureTeamName) {
	// 		secureExists = &filteredSec[i]
	// 		break
	// 	}
	// }

	// if secureExists == nil {
	// 	// 3a) Doesn’t exist → create it
	// 	secureID, err := helpers.CreateTeam(
	// 		apiEndpoint,
	// 		token,
	// 		facts.ContainerSecureTeamName,    // name
	// 		sysdigTeam.Spec.Team.Description, // description
	// 		"secure",                         // product
	// 		facts.Namespaces,                 // namespaces → used to build scopes
	// 	)
	// 	if err != nil {
	// 		return ctrl.Result{}, fmt.Errorf("create secure team: %w", err)
	// 	}
	// 	r.Log.Info("Created secure team", "id", secureID)

	// 	r.Log.Info("Assigned users to new secure team", "id", secureID)

	// } else {
	// 	// 4) Already exists → update it
	// 	// _, err = helpers.UpdateTeam(
	// 	// 	apiEndpoint,
	// 	// 	token,
	// 	// 	sysdigTeam.Spec.Team.Description,
	// 	// 	facts.ContainerSecureTeamName,
	// 	// 	secureExists.ID,
	// 	// 	// secureExists.Version,
	// 	// 	facts.Namespaces,
	// 	// )
	// 	if err != nil {
	// 		return ctrl.Result{}, fmt.Errorf("update secure team: %w", err)
	// 	}
	// 	r.Log.Info("Updated existing secure team", "id", secureExists.ID)
	// }

	// ------------------------------------------------------------------------------
	return ctrl.Result{}, nil
	// +++++++++++++++
}

// SetupWithManager sets up the controller with the Manager.
func (r *SysdigTeamGoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1alpha1.SysdigTeamGo{}).
		Complete(r)
}
