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

	opsv1alpha1 "github.com/bcgov/platform-services-sysdig/sysdig-operator/api/v1alpha1"
)

const sysdigTeamFinalizer = "monitoring.devops.gov.bc.ca/finalizer"
// This is the old finalizer and some SysdigTeams still have it, so we'll
//   remove it if it exists.
const sysdigTeamFinalizerOld = "finalizer.ops.gov.bc.ca"

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

		// After creating a team, create the associated dashboard.
		if product == "monitor" && len(namespaces) > 0 {
			dashboardApiEndpoint := os.Getenv("SYSDIG_DASHBOARD_API_ENDPOINT")
			if dashboardApiEndpoint == "" {
				dashboardApiEndpoint = "https://app.sysdigcloud.com" // Default if not set
			}

			r.Log.Info("Attempting to create default dashboard for new monitor team", "teamID", id)
			// We use the first namespace in the list for the dashboard scope.
			if err := helpers.CreateDashboard(dashboardApiEndpoint, token, id, namespaces[0]); err != nil {
				// Log the dashboard creation error as a warning but don't fail the reconciliation,
				// as the team itself was created successfully.
				r.Log.Error(err, "Warning: failed to create default dashboard for team", "teamID", id)
			} else {
				r.Log.Info("Successfully created default dashboard for team", "teamID", id)
			}
		}

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
	logger := log.FromContext(ctx) // Use a local logger

	apiEndpoint := os.Getenv("SYSDIG_API_ENDPOINT")
	token := os.Getenv("SYSDIG_TOKEN")

	logger.Info("Reconciling SysdigTeamGo", "Request.Namespace", req.Namespace, "Request.Name", req.Name)

	// Step 1: Fetch the CR instance
	var sysdigTeam api.SysdigTeam
	if err := r.Get(ctx, req.NamespacedName, &sysdigTeam); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("SysdigTeamGo resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get SysdigTeamGo resource")
		return ctrl.Result{}, err
	}

	// Handle deletion: Check if the DeletionTimestamp is set
	if !sysdigTeam.ObjectMeta.DeletionTimestamp.IsZero() {

		// If the old finalizer exists in a SysdigTeam, remove it.
		if containsString(sysdigTeam.ObjectMeta.Finalizers, sysdigTeamFinalizerOld) {
			logger.Info("Removing old finalizer...")
			sysdigTeam.ObjectMeta.Finalizers = removeString(sysdigTeam.ObjectMeta.Finalizers, sysdigTeamFinalizerOld)
			if err := r.Update(ctx, &sysdigTeam); err != nil {
				logger.Error(err, "Failed to remove OLD finalizer from SysdigTeamGo resource")
				return ctrl.Result{}, err
			}
			logger.Info("Successfully removed OLD finalizer")
		}

		if containsString(sysdigTeam.ObjectMeta.Finalizers, sysdigTeamFinalizer) {
			logger.Info("SysdigTeamGo resource is being deleted, performing cleanup...")

			// Remove the finalizer
			sysdigTeam.ObjectMeta.Finalizers = removeString(sysdigTeam.ObjectMeta.Finalizers, sysdigTeamFinalizer)
			if err := r.Update(ctx, &sysdigTeam); err != nil {
				logger.Error(err, "Failed to remove finalizer from SysdigTeamGo resource")
				return ctrl.Result{}, err
			}
			logger.Info("Successfully removed finalizer and cleaned up Sysdig teams.")

			// Perform cleanup: Delete Sysdig teams
			if sysdigTeam.Status.MonitorTeamID != 0 {
				logger.Info("Deleting Monitor team", "ID", sysdigTeam.Status.MonitorTeamID)
				if err := helpers.DeleteTeam(apiEndpoint, token, sysdigTeam.Status.MonitorTeamID); err != nil {
					// Log error but attempt to continue to delete the other team and remove finalizer
					logger.Error(err, "Failed to delete Monitor team", "ID", sysdigTeam.Status.MonitorTeamID)
				}
			}
			if sysdigTeam.Status.SecureTeamID != 0 {
				logger.Info("Deleting Secure team", "ID", sysdigTeam.Status.SecureTeamID)
				if err := helpers.DeleteTeam(apiEndpoint, token, sysdigTeam.Status.SecureTeamID); err != nil {
					logger.Error(err, "Failed to delete Secure team", "ID", sysdigTeam.Status.SecureTeamID)
				}
			}

		}
		return ctrl.Result{}, nil // Stop reconciliation as the object is being deleted
	}

	// Add finalizer if it doesn't exist
	if !containsString(sysdigTeam.ObjectMeta.Finalizers, sysdigTeamFinalizer) {
		sysdigTeam.ObjectMeta.Finalizers = append(sysdigTeam.ObjectMeta.Finalizers, sysdigTeamFinalizer)
		if err := r.Update(ctx, &sysdigTeam); err != nil {
			logger.Error(err, "Failed to add finalizer to SysdigTeam resource")
			return ctrl.Result{}, err
		}
		logger.Info("Added finalizer to SysdigTeamGo resource")
		return ctrl.Result{Requeue: true}, nil // Requeue to ensure status update occurs after finalizer addition
	}

	// Step 1.5 verify credentials
	if apiEndpoint == "" || token == "" {
		errMsg := "Environment variables SYSDIG_API_ENDPOINT and/or SYSDIG_TOKEN are not set"
		logger.Error(nil, errMsg) // Use logger for errors
		sysdigTeam.Status.Conditions = []api.Condition{
			{
				Type:    "SysdigCredentials",
				Status:  "False",
				Reason:  "MissingEnvVars",
				Message: errMsg,
			},
		}
		if err := r.Status().Update(ctx, &sysdigTeam); err != nil {
			logger.Error(err, "Failed to update SysdigTeamGo status for missing credentials")
		}
		return ctrl.Result{}, fmt.Errorf("%s", fmt.Sprintf("%s", errMsg)) // Return error to requeue
	}

	// STEP 2 verify if object is in tools namespace
	match, err := regexp.MatchString("(?i).*-tools$", req.Namespace)
	if err != nil {
		logger.Error(err, "Regex validation error for namespace")
		return ctrl.Result{}, fmt.Errorf("regex validation error: %v", err)
	}

	if !match {
		errMsg := "Object must be deployed in a namespace ending with '-tools'"
		logger.Info(errMsg, "Namespace", req.Namespace) // Log as info, not necessarily an error for the controller
		sysdigTeam.Status.Conditions = []api.Condition{
			{
				Type:    "NamespaceValidation",
				Status:  "False",
				Reason:  "InvalidNamespace",
				Message: errMsg,
			},
		}
		if err := r.Status().Update(ctx, &sysdigTeam); err != nil {
			logger.Error(err, "Failed to update SysdigTeamGo status for invalid namespace")
		}
		return ctrl.Result{}, nil // Don't requeue, this is a configuration issue
	}

	logger.Info("Namespace validation passed", "Namespace", req.Namespace)

	// Step 0: set fact (Moved after initial checks and finalizer logic)
	facts := helpers.SetTeamFacts(req.Namespace)

	// 3) Build teamUserList from the CR's spec
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
		logger.Error(err, "Failed to sync Monitor team")
		// Update status with error condition before returning
		sysdigTeam.Status.Conditions = []api.Condition{
			{Type: "Ready", Status: "False", Reason: "TeamSyncFailed", Message: "Failed to sync Monitor team: " + err.Error()},
		}
		if statusUpdateErr := r.Status().Update(ctx, &sysdigTeam); statusUpdateErr != nil {
			logger.Error(statusUpdateErr, "Failed to update status for Monitor team sync failure")
		}
		return ctrl.Result{}, err
	}
	sysdigTeam.Status.MonitorTeamID = monitorTeamID // Store MonitorTeamID

	if err := r.syncMemberships(apiEndpoint, token, monitorTeamID, teamUsersAndRoles, "monitor"); err != nil {
		logger.Error(err, "Failed to sync Monitor team memberships")
		sysdigTeam.Status.Conditions = []api.Condition{
			{Type: "Ready", Status: "False", Reason: "MembershipSyncFailed", Message: "Failed to sync Monitor team memberships: " + err.Error()},
		}
		if statusUpdateErr := r.Status().Update(ctx, &sysdigTeam); statusUpdateErr != nil {
			logger.Error(statusUpdateErr, "Failed to update status for Monitor team membership sync failure")
		}
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
		logger.Error(err, "Failed to sync Secure team")
		sysdigTeam.Status.Conditions = []api.Condition{
			{Type: "Ready", Status: "False", Reason: "TeamSyncFailed", Message: "Failed to sync Secure team: " + err.Error()},
		}
		if statusUpdateErr := r.Status().Update(ctx, &sysdigTeam); statusUpdateErr != nil {
			logger.Error(statusUpdateErr, "Failed to update status for Secure team sync failure")
		}
		return ctrl.Result{}, err
	}
	sysdigTeam.Status.SecureTeamID = secureTeamID // Store SecureTeamID

	logger.Info("Successfully synced teams", "MonitorTeamID", monitorTeamID, "SecureTeamID", secureTeamID)

	if err := r.syncMemberships(apiEndpoint, token, secureTeamID, teamUsersAndRoles, "secure"); err != nil {
		logger.Error(err, "Failed to sync Secure team memberships")
		sysdigTeam.Status.Conditions = []api.Condition{
			{Type: "Ready", Status: "False", Reason: "MembershipSyncFailed", Message: "Failed to sync Secure team memberships: " + err.Error()},
		}
		if statusUpdateErr := r.Status().Update(ctx, &sysdigTeam); statusUpdateErr != nil {
			logger.Error(statusUpdateErr, "Failed to update status for Secure team membership sync failure")
		}
		return ctrl.Result{}, err
	}
	// 7) Assign or update memberships for each user in both teams - THIS SECTION SEEMS REDUNDANT
	// The syncMemberships function already ensures the desired state.
	// The loop below re-applies SaveMembership, which might be okay for idempotency but syncMemberships should handle it.
	// I will comment it out for now as syncMemberships is designed to be the source of truth for memberships.
	/*
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
	*/

	// Update status to Ready
	sysdigTeam.Status.Conditions = []api.Condition{
		{
			Type:    "Ready",
			Status:  "True",
			Reason:  "Reconciled",
			Message: "Sysdig teams and memberships reconciled successfully",
		},
	}
	if err := r.Status().Update(ctx, &sysdigTeam); err != nil {
		logger.Error(err, "Failed to update SysdigTeam status to Ready")
		return ctrl.Result{}, err
	}

	logger.Info("Successfully reconciled SysdigTeam")
	return ctrl.Result{}, nil
}

// containsString checks if a slice of strings contains a specific string.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// removeString removes a specific string from a slice of strings.
func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// SetupWithManager sets up the controller with the Manager.
func (r *SysdigTeamGoReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log = ctrl.Log.WithName("controllers").WithName("SysdigTeam")
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsv1alpha1.SysdigTeam{}).
		Complete(r)
}
