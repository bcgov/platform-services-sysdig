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

func (r *SysdigTeamGoReconciler) syncOneTeam(
	apiEndpoint, token, teamName, product, description string,
	namespaces []string,
) (int64, error) {
	// Fetch existing teams by name filter
	filtered, err := helpers.FetchTeams(apiEndpoint, token, teamName)

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

// syncMemberships ensures the given teamID has exactly the desired
// user/role pairs. It only calls SaveMembership when a user is missing
// or has the wrong role.
func (r *SysdigTeamGoReconciler) syncMemberships(
	apiEndpoint, token string,
	teamID int64,
	desired []helpers.TeamUserRole,
	product string,
) error {
	existing, err := helpers.FetchTeamMemberships(apiEndpoint, token, teamID)
	if err != nil {
		return fmt.Errorf("fetch %s memberships: %w", product, err)
	}

	// build lookup: userID -> role
	existMap := make(map[int64]string, len(existing))
	for _, m := range existing {
		existMap[m.UserID] = m.Role
	}

	desiredMap := make(map[int64]string, len(desired))
	for _, d := range desired {
		desiredMap[d.UserID] = d.Role
	}
	for _, d := range desired {
		currentRole, found := existMap[d.UserID]

		switch {
		case !found:
			// never had this user — just create
			resp, err := helpers.SaveMembership(apiEndpoint, token,
				teamID, d.UserID, d.Role)
			if err != nil {
				r.Log.Error(err, "SaveMembership failed (new)",
					"team", product, "teamID", teamID,
					"userID", d.UserID, "role", d.Role)
			} else {
				r.Log.Info("SaveMembership succeeded (new)",
					"team", product, "teamID", teamID,
					"userID", d.UserID, "role", d.Role,
					"response", string(resp))
			}

		case currentRole != d.Role:
			// role changed — delete then re‐create (Role check)
			if err := helpers.DeleteMembership(apiEndpoint, token, teamID, d.UserID); err != nil {
				r.Log.Error(err, "DeleteMembership failed",
					"team", product, "teamID", teamID,
					"userID", d.UserID, "oldRole", currentRole)
			} else {
				r.Log.Info("DeleteMembership succeeded",
					"team", product, "teamID", teamID,
					"userID", d.UserID, "oldRole", currentRole)
			}

			// now create with the new role
			resp, err := helpers.SaveMembership(apiEndpoint, token,
				teamID, d.UserID, d.Role)
			if err != nil {
				r.Log.Error(err, "SaveMembership failed (after delete)",
					"team", product, "teamID", teamID,
					"userID", d.UserID, "newRole", d.Role)
			} else {
				r.Log.Info("SaveMembership succeeded (after delete)",
					"team", product, "teamID", teamID,
					"userID", d.UserID, "role", d.Role,
					"response", string(resp))
			}

		default:
			// found and role matches — nothing to do
			r.Log.Info("Membership already correct",
				"team", product, "teamID", teamID,
				"userID", d.UserID, "role", d.Role)
		}
	}

	// Remove any extra users not in desired list (ID check)
	for _, m := range existing {
		if _, keep := desiredMap[m.UserID]; !keep {

			// Dustin says we can not delete ROLE_TEAM_MAGAGER once they been added, even this user does not exist
			// , so we should let user stop adding this role.
			if m.Role == "ROLE_TEAM_MANAGER" {
				r.Log.Info("Skipping deletion of manager",
					"team", product, "teamID", teamID, "userID", m.UserID)
				continue
			}
			if err := helpers.DeleteMembership(apiEndpoint, token, teamID, m.UserID); err != nil {

				r.Log.Error(err, "DeleteMembership failed",
					"team", product, "teamID", teamID, "userID", m.UserID, "role", m.Role)
			} else {
				r.Log.Info("Deleted extra membership",
					"team", product, "teamID", teamID, "userID", m.UserID, "role", m.Role)
			}
		}
	}
	return nil
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

	apiEndpoint := os.Getenv("SYSDIG_API_ENDPOINT")
	token := os.Getenv("SYSDIG_TOKEN")

	fmt.Printf("DEBUG: Reconciling SysdigTeamGo: %s\n", req.NamespacedName)

	// Step 0: set fact
	facts := helpers.SetTeamFacts(req.Namespace)

	// Step 1: Fetch the CR instance
	var sysdigTeam api.SysdigTeam
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

	// 5)----- MONITOR TEAM -----
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

	// make sure membership is the same as what sysdig have
	if err := r.syncMemberships(apiEndpoint, token, monitorTeamID, teamUsersAndRoles, "monitor"); err != nil {
		return ctrl.Result{}, err
	}

	// 6) ----- Secure TEAM -----
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
	// make sure membership is the same as what sysdig have
	if err := r.syncMemberships(apiEndpoint, token, secureTeamID, teamUsersAndRoles, "secure"); err != nil {
		return ctrl.Result{}, err
	}
	// 7) Assign or update memberships for each user in both teams
	for _, m := range teamUsersAndRoles {
		// Monitor team membership

		_, Monitorerr := helpers.SaveMembership(
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
		_, Secureerr := helpers.SaveMembership(
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

		r.Log.Info("finish user", "name", m.Name)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SysdigTeamGoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log = ctrl.Log.WithName("controllers").WithName("SysdigTeam")
	return ctrl.NewControllerManagedBy(mgr).
		For(&monitoringv1alpha1.SysdigTeam{}).
		Complete(r)
}
