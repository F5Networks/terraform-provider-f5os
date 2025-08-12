package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	f5os "gitswarm.f5net.com/terraform-providers/f5osclient"
)

type UserResource struct {
	client *f5os.F5os
}

func NewUserResource() resource.Resource {
	return &UserResource{}
}

func (r *UserResource) Metadata(_ context.Context, _ resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = "f5os_user"
}

func (r *UserResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData != nil {
		r.client = req.ProviderData.(*f5os.F5os)
	}
}

func (r *UserResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage users and roles on F5OS-based systems (Velos controller or rSeries appliance)",
		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				MarkdownDescription: "Specifies assigned username of user.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "Password for the user account. Must meet F5OS device password policy requirements.",
				Required:            true,
				Sensitive:           true,
				Validators:          []validator.String{basicPasswordValidator{}},
			},
			"role": schema.StringAttribute{
				MarkdownDescription: "Specifies primary role assigned to the user (e.g., admin, operator, user).",
				Required:            true,
			},
			"secondary_role": schema.StringAttribute{
				MarkdownDescription: "Optional secondary role assigned to the user.",
				Optional:            true,
			},
			"authorized_keys": schema.ListAttribute{
				MarkdownDescription: "List of SSH authorized keys for the user (optional).",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"expiry_status": schema.StringAttribute{
				MarkdownDescription: "Account expiration status. Value can be 'enabled', 'locked', or a specific expiry date in YYYY-MM-DD format.",
				Optional:            true,
				Validators:          []validator.String{expiryStatusValidator{}},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform synthetic ID (username).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *UserResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan UserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	username := plan.Username.ValueString()

	// Try to create the user with all fields including secondary role
	err := r.createUserWithRetry(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("User Create Error", err.Error())
		return
	}

	// Set the password using the set-password endpoint
	if !plan.Password.IsNull() && !plan.Password.IsUnknown() && plan.Password.ValueString() != "" {
		password := plan.Password.ValueString()

		err = r.setUserPassword(ctx, username, password)
		if err != nil {
			// If password setting fails, clean up the created user
			deleteErr := r.deleteUser(username)
			if deleteErr != nil {
				tflog.Error(ctx, "Failed to clean up user after password error", map[string]interface{}{
					"username":     username,
					"delete_error": deleteErr.Error(),
				})
			}

			resp.Diagnostics.AddError(
				"Error Setting User Password",
				"Could not set password for user "+username+": "+err.Error(),
			)
			return
		}

		tflog.Info(ctx, "Password set successfully for user", map[string]interface{}{
			"username": username,
		})
	}

	tflog.Info(ctx, "User created successfully", map[string]any{
		"username": username,
		"role":     plan.Role.ValueString(),
	})

	plan.ID = plan.Username
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *UserResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.getUser(state.Username.ValueString())
	if err != nil {
		// If user not found, remove from state
		if err.Error() == "user not found" {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("User Read Error", err.Error())
		return
	}

	// Preserve passwords from current state (they are not returned by API)
	// This is critical for idempotency - passwords should never be read from API

	state.ID = types.StringValue(state.Username.ValueString())
	state.Username = types.StringValue(user.Username)

	// Always set role from API - this is required and should always be present
	if user.Role != "" {
		state.Role = types.StringValue(user.Role)
	} else {
		// If role is empty from API but we have it in state, preserve it
		// During import, this might be empty, so we should set an error
		if state.Role.IsNull() || state.Role.IsUnknown() {
			tflog.Error(ctx, "Role not found in API response and not in state - this user may not have any role assignments", map[string]any{
				"username": user.Username,
			})
			resp.Diagnostics.AddError("User Role Error",
				fmt.Sprintf("User '%s' does not have any role assignments. Every user must have at least one role.", user.Username))
			return
		}
		// Keep existing role from state if API doesn't return it
		tflog.Warn(ctx, "Role not returned by API, preserving from state", map[string]any{
			"username":   user.Username,
			"state_role": state.Role.ValueString(),
		})
	}

	// Set secondary role if present in API, otherwise preserve current state for idempotency
	if user.SecondaryRole != "" {
		state.SecondaryRole = types.StringValue(user.SecondaryRole)
	}
	// If API doesn't return secondary role, preserve whatever is in state
	// This ensures idempotency - we don't change the state unless API explicitly tells us to

	// Set authorized keys if present in API, otherwise preserve current state for idempotency
	if len(user.AuthorizedKeys) > 0 {
		keyValues := make([]types.String, len(user.AuthorizedKeys))
		for i, key := range user.AuthorizedKeys {
			keyValues[i] = types.StringValue(key)
		}
		keysList, _ := types.ListValueFrom(ctx, types.StringType, keyValues)
		state.AuthorizedKeys = keysList
	}
	// If API doesn't return authorized keys, preserve whatever is in state
	// This ensures idempotency - we don't change the state unless API explicitly tells us to

	// Set expiry status if present in API, otherwise preserve current state for idempotency
	if user.ExpiryStatus != "" {
		state.ExpiryStatus = types.StringValue(user.ExpiryStatus)
	}
	// If API doesn't return expiry status, preserve whatever is in state
	// This ensures idempotency - we don't change the state unless API explicitly tells us to

	tflog.Debug(ctx, "Current Read Result", map[string]any{
		"username":            user.Username,
		"role":                user.Role,
		"secondary_role":      user.SecondaryRole,
		"authorized_keys":     user.AuthorizedKeys,
		"expiry_status":       user.ExpiryStatus,
		"id":                  state.ID,
		"password_maintained": "passwords preserved from state",
		"state_before_update": map[string]any{
			"state_username":       state.Username.ValueString(),
			"state_role":           state.Role.ValueString(),
			"state_secondary_role": state.SecondaryRole.ValueString(),
		},
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *UserResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan UserModel
	var state UserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	username := plan.Username.ValueString()

	// Check if user exists before updating
	_, err := r.getUser(username)
	if err != nil {
		if err.Error() == "user not found" {
			resp.Diagnostics.AddError("User Update Error",
				fmt.Sprintf("User '%s' does not exist and cannot be updated", username))
		} else {
			resp.Diagnostics.AddError("User Update Error",
				fmt.Sprintf("Failed to verify user existence: %s", err.Error()))
		}
		return
	}

	// Step 1: Update user attributes (without password)
	err = r.updateUserWithRetry(ctx, username, plan)
	if err != nil {
		resp.Diagnostics.AddError("User Update Error", err.Error())
		return
	}

	// Step 2: Update password if it changed
	passwordChanged := false
	if !plan.Password.IsNull() && !plan.Password.IsUnknown() {
		newPassword := plan.Password.ValueString()
		// For password changes, we need the old password - use the state password
		oldPassword := ""
		if !state.Password.IsNull() && !state.Password.IsUnknown() {
			oldPassword = state.Password.ValueString()
		}

		// Only update password if it actually changed
		if newPassword != oldPassword {
			passwordChanged = true
			err = r.changeUserPassword(ctx, username, oldPassword, newPassword)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Updating User Password",
					"Could not update password for user "+username+": "+err.Error(),
				)
				return
			}

			tflog.Info(ctx, "Password updated successfully for user", map[string]interface{}{
				"username": username,
			})
		}
	}

	tflog.Info(ctx, "Updated User", map[string]any{
		"username":         username,
		"role":             plan.Role.ValueString(),
		"password_changed": passwordChanged,
	})

	plan.ID = plan.Username
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state UserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	username := state.Username.ValueString()

	// Check if user exists before attempting deletion
	_, err := r.getUser(username)
	if err != nil {
		if err.Error() == "user not found" {
			// User already doesn't exist, consider this successful
			tflog.Info(ctx, "User already deleted or does not exist", map[string]any{
				"username": username,
			})
			return
		}
		resp.Diagnostics.AddError("User Delete Error",
			fmt.Sprintf("Failed to verify user existence before deletion: %s", err.Error()))
		return
	}

	// Step 1: Remove user from all role assignments before deleting
	userRoles, err := r.getUserRoles(ctx, username)
	if err != nil {
		tflog.Warn(ctx, "Failed to get user roles during deletion, will try direct deletion", map[string]any{
			"username": username,
			"error":    err.Error(),
		})
	} else {
		// Remove user from all roles (both primary and secondary)
		for _, role := range userRoles {
			tflog.Info(ctx, "Removing user from role before deletion", map[string]any{
				"username": username,
				"role":     role,
			})

			err = r.removeUserFromRole(ctx, username, role)
			if err != nil {
				tflog.Warn(ctx, "Failed to remove user from role, continuing with deletion", map[string]any{
					"username": username,
					"role":     role,
					"error":    err.Error(),
				})
			}
		}
	}

	// Step 2: Delete the user
	if err := r.deleteUser(username); err != nil {
		resp.Diagnostics.AddError("User Delete Error", err.Error())
		return
	}

	tflog.Info(ctx, "Deleted User", map[string]any{
		"username": username,
	})
}

func (r *UserResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Use the ID as the username
	resource.ImportStatePassthroughID(ctx, path.Root("username"), req, resp)

	// Set the ID to the same value as username for consistency
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)

	tflog.Info(ctx, "Importing User", map[string]any{
		"username": req.ID,
	})

	// Note: The Read function will be called automatically after import to populate the state
	// This ensures all fields including role are properly set from the API
}

// UserStruct is the internal Go representation for HTTP payloads
type UserStruct struct {
	Username       string   `json:"username"`
	Role           string   `json:"role"`
	SecondaryRole  string   `json:"secondary-role,omitempty"`
	AuthorizedKeys []string `json:"authorized-keys,omitempty"`
	ExpiryStatus   string   `json:"expiry-status,omitempty"`
	// Try additional field mappings in case F5OS uses different names
	F5Role   string `json:"f5-system-aaa:role,omitempty"`
	UserRole string `json:"user-role,omitempty"`
}

// userConfig represents the config section in the API payload
type userConfig struct {
	Username       string   `json:"username"`
	Password       string   `json:"password,omitempty"`
	Role           string   `json:"role"`
	SecondaryRole  string   `json:"secondary-role,omitempty"`
	AuthorizedKeys []string `json:"authorized-keys,omitempty"`
	ExpiryStatus   string   `json:"expiry-status,omitempty"`
}

// userPayload represents the full API payload structure
type userPayload struct {
	User struct {
		Username string     `json:"username"`
		Password string     `json:"password,omitempty"`
		Config   userConfig `json:"config"`
	} `json:"f5-system-aaa:user"`
}

// userResponseWrapper represents the API response structure
type userResponseWrapper struct {
	Users []UserStruct `json:"f5-system-aaa:user"`
}

// passwordChangePayload represents the password change API payload
type passwordChangePayload struct {
	OldPassword string `json:"f5-system-aaa:old-password,omitempty"`
	NewPassword string `json:"f5-system-aaa:new-password"`
}

// createUserPayload creates the JSON payload for user creation/update (including password for new users)
func (r *UserResource) createUserPayload(plan UserModel, includePassword bool, includeSecondaryRole bool) ([]byte, error) {
	payload := userPayload{}
	payload.User.Username = plan.Username.ValueString()
	payload.User.Config.Username = plan.Username.ValueString()
	payload.User.Config.Role = plan.Role.ValueString()

	// Include password for new user creation (not for updates)
	if includePassword && !plan.Password.IsNull() && !plan.Password.IsUnknown() {
		payload.User.Config.Password = plan.Password.ValueString()
	}

	// Add secondary role if specified and enabled
	if includeSecondaryRole && !plan.SecondaryRole.IsNull() && !plan.SecondaryRole.IsUnknown() {
		payload.User.Config.SecondaryRole = plan.SecondaryRole.ValueString()
	}

	// Add authorized keys if specified
	if !plan.AuthorizedKeys.IsNull() && !plan.AuthorizedKeys.IsUnknown() {
		var keys []string
		diags := plan.AuthorizedKeys.ElementsAs(context.Background(), &keys, false)
		if !diags.HasError() {
			payload.User.Config.AuthorizedKeys = keys
		}
	}

	// Add expiry status if specified
	if !plan.ExpiryStatus.IsNull() && !plan.ExpiryStatus.IsUnknown() {
		payload.User.Config.ExpiryStatus = plan.ExpiryStatus.ValueString()
	}

	return json.Marshal(payload)
}

// getUser retrieves user information via API and determines secondary roles from roles endpoint
func (r *UserResource) getUser(username string) (*UserStruct, error) {
	uri := fmt.Sprintf("/openconfig-system:system/aaa/authentication/f5-system-aaa:users/user=%s", username)

	respData, err := r.client.GetRequest(uri)
	if err != nil {
		// Check if this is a 404 error (user not found)
		errStr := fmt.Sprintf("%s", err)
		if errStr == "404" ||
			strings.Contains(errStr, "404") ||
			strings.Contains(errStr, "not found") {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	var response userResponseWrapper
	if err := json.Unmarshal(respData, &response); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	tflog.Debug(context.Background(), "Get User API Response", map[string]any{
		"raw_response": string(respData),
		"parsed_users": response.Users,
		"users_count":  len(response.Users),
	})

	if len(response.Users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	user := &response.Users[0]

	// Handle potential alternative field names for role
	if user.Role == "" && user.F5Role != "" {
		user.Role = user.F5Role
	}
	if user.Role == "" && user.UserRole != "" {
		user.Role = user.UserRole
	}

	// Get all roles for this user to determine primary and secondary roles
	userRoles, err := r.getUserRoles(context.Background(), username)
	if err != nil {
		tflog.Warn(context.Background(), "Failed to get user roles, will use role from user config if available", map[string]any{
			"username": username,
			"error":    err.Error(),
		})
	} else {
		// If user has no role in their config but has role assignments, use the first role as primary
		if user.Role == "" && len(userRoles) > 0 {
			user.Role = userRoles[0]
			tflog.Debug(context.Background(), "Using first role assignment as primary role", map[string]any{
				"username":     username,
				"primary_role": user.Role,
				"all_roles":    userRoles,
			})
		}

		// Find secondary role (any role that's not the primary role)
		for _, role := range userRoles {
			if role != user.Role {
				user.SecondaryRole = role
				break // Take the first non-primary role as secondary role
			}
		}
	}

	tflog.Debug(context.Background(), "Retrieved User Details", map[string]any{
		"username":              user.Username,
		"role":                  user.Role,
		"f5_role":               user.F5Role,
		"user_role":             user.UserRole,
		"secondary_role":        user.SecondaryRole,
		"all_roles":             userRoles,
		"authorized_keys_count": len(user.AuthorizedKeys),
		"expiry_status":         user.ExpiryStatus,
	})

	return user, nil
}

// updateUser updates an existing user via API
func (r *UserResource) updateUser(username string, payload []byte) error {
	uri := fmt.Sprintf("/openconfig-system:system/aaa/authentication/f5-system-aaa:users/user=%s", username)

	respData, err := r.client.PatchRequest(uri, payload)
	if err != nil {
		// Check if this is a password policy violation
		errStr := fmt.Sprintf("%s", err)
		if strings.Contains(errStr, "BAD PASSWORD") ||
			strings.Contains(errStr, "password policy") ||
			strings.Contains(errStr, "dictionary check") ||
			strings.Contains(errStr, "simplistic") ||
			strings.Contains(errStr, "systematic") {
			return fmt.Errorf("password does not meet F5OS device policy requirements: %w", err)
		}
		return fmt.Errorf("API request failed: %w", err)
	}

	tflog.Debug(context.Background(), "Update User Response", map[string]any{
		"response": string(respData),
	})

	return nil
}

// deleteUser deletes a user via API
func (r *UserResource) deleteUser(username string) error {
	uri := fmt.Sprintf("/openconfig-system:system/aaa/authentication/f5-system-aaa:users/user=%s", username)

	err := r.client.DeleteRequest(uri)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	return nil
}

// changeUserPassword changes the password for an existing user using the change-password API
func (r *UserResource) changeUserPassword(ctx context.Context, username, oldPassword, newPassword string) error {
	tflog.Debug(ctx, "Changing User Password", map[string]any{
		"username":   username,
		"platform":   r.client.PlatformType,
		"hasOldPass": oldPassword != "",
	})

	// Use the change-password endpoint for password changes
	uri := fmt.Sprintf("/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=%s/f5-system-aaa:config/f5-system-aaa:change-password", username)
	passwordPayload := passwordChangePayload{
		OldPassword: oldPassword,
		NewPassword: newPassword,
	}

	payload, err := json.Marshal(passwordPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal password payload: %w", err)
	}

	tflog.Debug(ctx, "Password Change API Request", map[string]any{
		"username": username,
		"uri":      uri,
		"payload":  string(payload),
	})

	respData, err := r.client.PostRequest(uri, payload)
	if err != nil {
		errStr := err.Error()
		tflog.Debug(ctx, "Password Change API Error", map[string]any{
			"error": errStr,
		})

		// Check for F5OS password policy violations
		if strings.Contains(errStr, "password") &&
			(strings.Contains(errStr, "length") ||
				strings.Contains(errStr, "character") ||
				strings.Contains(errStr, "dictionary check") ||
				strings.Contains(errStr, "simplistic") ||
				strings.Contains(errStr, "systematic")) {
			return fmt.Errorf("password does not meet F5OS device policy requirements: %w", err)
		}
		return fmt.Errorf("API request failed: %w", err)
	}

	tflog.Debug(ctx, "Change User Password Response", map[string]any{
		"username": username,
		"response": string(respData),
	})

	return nil
}

// setUserPassword sets the password for a user using the set-password API
func (r *UserResource) setUserPassword(ctx context.Context, username, password string) error {
	uri := fmt.Sprintf("/openconfig-system:system/aaa/authentication/f5-system-aaa:users/f5-system-aaa:user=%s/f5-system-aaa:config/f5-system-aaa:set-password", username)

	// Use the correct payload structure from the API documentation
	passwordPayload := map[string]string{
		"f5-system-aaa:password": password,
	}

	payload, err := json.Marshal(passwordPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal password payload: %w", err)
	}

	tflog.Debug(ctx, "Setting User Password", map[string]any{
		"username": username,
		"uri":      uri,
		"payload":  string(payload),
	})

	respData, err := r.client.PostRequest(uri, payload)
	if err != nil {
		errStr := err.Error()
		tflog.Debug(ctx, "Set Password API Error", map[string]any{
			"error": errStr,
		})

		// Check for F5OS password policy violations
		if strings.Contains(errStr, "password") &&
			(strings.Contains(errStr, "length") ||
				strings.Contains(errStr, "character") ||
				strings.Contains(errStr, "dictionary check") ||
				strings.Contains(errStr, "simplistic") ||
				strings.Contains(errStr, "systematic")) {
			return fmt.Errorf("password does not meet F5OS device policy requirements: %w", err)
		}
		return fmt.Errorf("API request failed: %w", err)
	}

	tflog.Debug(ctx, "Set User Password Response", map[string]any{
		"username": username,
		"response": string(respData),
	})

	return nil
}

// createUser creates a new user via API using POST to the users collection
func (r *UserResource) createUser(payload []byte) error {
	uri := "/openconfig-system:system/aaa/authentication/f5-system-aaa:users"

	tflog.Debug(context.Background(), "Creating User via POST", map[string]any{
		"uri":     uri,
		"payload": string(payload),
	})

	respData, err := r.client.PostRequest(uri, payload)
	if err != nil {
		// Check if this is a password policy violation
		errStr := fmt.Sprintf("%s", err)
		if strings.Contains(errStr, "BAD PASSWORD") ||
			strings.Contains(errStr, "password policy") ||
			strings.Contains(errStr, "dictionary check") ||
			strings.Contains(errStr, "simplistic") ||
			strings.Contains(errStr, "systematic") {
			return fmt.Errorf("password does not meet F5OS device policy requirements: %w", err)
		}
		return fmt.Errorf("API request failed: %w", err)
	}

	tflog.Debug(context.Background(), "Create User Response", map[string]any{
		"response": string(respData),
	})

	return nil
}

// createUserWithRetry creates a user and assigns secondary role via separate endpoint
func (r *UserResource) createUserWithRetry(ctx context.Context, plan UserModel) error {
	username := plan.Username.ValueString()
	primaryRole := plan.Role.ValueString()

	// Step 1: Create the user with primary role (never include secondary role in user config)
	payload, err := r.createUserPayload(plan, false, false)
	if err != nil {
		return fmt.Errorf("failed to create payload: %w", err)
	}

	tflog.Info(ctx, "Creating F5OS user with primary role", map[string]interface{}{
		"username": username,
		"role":     primaryRole,
		"payload":  string(payload),
	})

	err = r.createUser(payload)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Step 2: Assign user to primary role via roles endpoint (this ensures they appear in role membership)
	tflog.Info(ctx, "Assigning user to primary role", map[string]interface{}{
		"username":     username,
		"primary_role": primaryRole,
	})

	err = r.assignUserToRole(ctx, username, primaryRole)
	if err != nil {
		tflog.Warn(ctx, "Failed to assign user to primary role, user created but role assignment may be incomplete",
			map[string]interface{}{
				"username":     username,
				"primary_role": primaryRole,
				"error":        err.Error(),
			})
	}

	// Step 3: Assign secondary role if specified (using roles endpoint)
	if !plan.SecondaryRole.IsNull() && !plan.SecondaryRole.IsUnknown() && plan.SecondaryRole.ValueString() != "" {
		secondaryRole := plan.SecondaryRole.ValueString()

		tflog.Info(ctx, "Assigning secondary role to user", map[string]interface{}{
			"username":       username,
			"secondary_role": secondaryRole,
		})

		err = r.assignUserToRole(ctx, username, secondaryRole)
		if err != nil {
			// If secondary role assignment fails, warn but don't fail the creation
			tflog.Warn(ctx, "Failed to assign secondary role, user created with primary role only",
				map[string]interface{}{
					"username":       username,
					"secondary_role": secondaryRole,
					"error":          err.Error(),
				})
		}
	}

	return nil
}

// updateUserWithRetry updates a user and manages secondary role via separate endpoint
func (r *UserResource) updateUserWithRetry(ctx context.Context, username string, plan UserModel) error {
	primaryRole := plan.Role.ValueString()

	// Step 1: Update user attributes with primary role (never include secondary role in user config)
	payload, err := r.createUserPayload(plan, false, false)
	if err != nil {
		return fmt.Errorf("failed to create payload: %w", err)
	}

	tflog.Info(ctx, "Updating F5OS user with primary role", map[string]interface{}{
		"username": username,
		"role":     primaryRole,
	})

	err = r.updateUser(username, payload)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	// Step 2: Ensure user is assigned to primary role via roles endpoint
	tflog.Info(ctx, "Ensuring user is assigned to primary role", map[string]interface{}{
		"username":     username,
		"primary_role": primaryRole,
	})

	err = r.assignUserToRole(ctx, username, primaryRole)
	if err != nil {
		tflog.Warn(ctx, "Failed to assign user to primary role during update",
			map[string]interface{}{
				"username":     username,
				"primary_role": primaryRole,
				"error":        err.Error(),
			})
	}

	// Step 3: Manage secondary role assignment
	if !plan.SecondaryRole.IsNull() && !plan.SecondaryRole.IsUnknown() && plan.SecondaryRole.ValueString() != "" {
		secondaryRole := plan.SecondaryRole.ValueString()

		tflog.Info(ctx, "Assigning secondary role to user", map[string]interface{}{
			"username":       username,
			"secondary_role": secondaryRole,
		})

		err = r.assignUserToRole(ctx, username, secondaryRole)
		if err != nil {
			// If secondary role assignment fails, warn but don't fail the update
			tflog.Warn(ctx, "Failed to assign secondary role during update",
				map[string]interface{}{
					"username":       username,
					"secondary_role": secondaryRole,
					"error":          err.Error(),
				})
		}
	} else {
		// If secondary role is being removed, we might need to unassign it
		// For now, we'll just log this - implementing role removal would require
		// knowing the previous secondary role to remove the assignment
		tflog.Info(ctx, "Secondary role not specified or being removed", map[string]interface{}{
			"username": username,
		})
	}

	return nil
}

// assignUserToRole assigns a user to a specific role using the roles endpoint
func (r *UserResource) assignUserToRole(ctx context.Context, username, roleName string) error {
	uri := fmt.Sprintf("/openconfig-system:system/aaa/authentication/f5-system-aaa:roles/f5-system-aaa:role=%s/f5-system-aaa:config/f5-system-aaa:users=%s", roleName, username)

	// Create the payload structure for role assignment
	rolePayload := map[string][]string{
		"f5-system-aaa:users": {username},
	}

	payload, err := json.Marshal(rolePayload)
	if err != nil {
		return fmt.Errorf("failed to marshal role assignment payload: %w", err)
	}

	tflog.Debug(ctx, "Assigning User to Role", map[string]any{
		"username": username,
		"role":     roleName,
		"uri":      uri,
		"payload":  string(payload),
	})

	// Use PUT request to assign the user to the role
	respData, err := r.client.PutRequest(uri, payload)
	if err != nil {
		errStr := err.Error()
		tflog.Debug(ctx, "Role Assignment API Error", map[string]any{
			"error": errStr,
		})

		// Check if the role doesn't exist
		if strings.Contains(errStr, "404") || strings.Contains(errStr, "not found") {
			return fmt.Errorf("role '%s' does not exist on the F5OS device", roleName)
		}

		return fmt.Errorf("API request failed: %w", err)
	}

	tflog.Debug(ctx, "User Role Assignment Response", map[string]any{
		"username": username,
		"role":     roleName,
		"response": string(respData),
	})

	return nil
}

// getUserRoles retrieves the roles assigned to a user from the roles endpoint
func (r *UserResource) getUserRoles(ctx context.Context, username string) ([]string, error) {
	uri := "/openconfig-system:system/aaa/authentication/f5-system-aaa:roles"

	respData, err := r.client.GetRequest(uri)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles: %w", err)
	}

	tflog.Debug(ctx, "Get All Roles API Response", map[string]any{
		"raw_response": string(respData),
	})

	// Parse the roles response to find which roles contain this user
	var rolesResponse struct {
		Roles struct {
			Role []struct {
				RoleName string `json:"rolename"`
				Config   struct {
					Users []string `json:"users,omitempty"`
				} `json:"config"`
			} `json:"role"`
		} `json:"f5-system-aaa:roles"`
	}

	if err := json.Unmarshal(respData, &rolesResponse); err != nil {
		return nil, fmt.Errorf("failed to parse roles response: %w", err)
	}

	tflog.Debug(ctx, "Parsed Roles Response Structure", map[string]any{
		"total_roles": len(rolesResponse.Roles.Role),
		"username":    username,
	})

	for i, role := range rolesResponse.Roles.Role {
		tflog.Debug(ctx, "Checking Role for User", map[string]any{
			"role_index": i,
			"role_name":  role.RoleName,
			"users":      role.Config.Users,
			"user_count": len(role.Config.Users),
		})
	}

	var userRoles []string
	for _, role := range rolesResponse.Roles.Role {
		for _, user := range role.Config.Users {
			if user == username {
				userRoles = append(userRoles, role.RoleName)
				break
			}
		}
	}

	tflog.Debug(ctx, "Found User Roles", map[string]any{
		"username": username,
		"roles":    userRoles,
	})

	return userRoles, nil
}

// UserModel maps Terraform resource attributes to F5OS API fields.
// It is used for state management and API payload construction.
type UserModel struct {
	ID             types.String `tfsdk:"id"`
	Username       types.String `tfsdk:"username"`
	Password       types.String `tfsdk:"password"`
	Role           types.String `tfsdk:"role"`
	SecondaryRole  types.String `tfsdk:"secondary_role"`
	AuthorizedKeys types.List   `tfsdk:"authorized_keys"`
	ExpiryStatus   types.String `tfsdk:"expiry_status"`
}

// Custom validator for expiry_status attribute
type expiryStatusValidator struct{}

// Ensure expiryStatusValidator implements validator.String
var _ validator.String = expiryStatusValidator{}

// Description provides a human-readable description of the validator
func (v expiryStatusValidator) Description(ctx context.Context) string {
	return "Ensures the expiry_status value is either 'enabled', 'locked', or a valid date in YYYY-MM-DD format."
}

// MarkdownDescription provides a markdown description of the validator
func (v expiryStatusValidator) MarkdownDescription(ctx context.Context) string {
	return "Ensures the expiry_status value is either `enabled`, `locked`, or a valid date in `YYYY-MM-DD` format."
}

// ValidateString performs the validation
func (v expiryStatusValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return // Optional field, null/unknown values are valid
	}

	input := req.ConfigValue.ValueString()

	// Check for predefined values
	if input == "enabled" || input == "locked" {
		return // Valid predefined values
	}

	// Check for YYYY-MM-DD date format and validate as actual date
	dateRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	if dateRegex.MatchString(input) {
		// Parse the date to ensure it's a valid calendar date
		if _, err := time.Parse("2006-01-02", input); err != nil {
			resp.Diagnostics.AddError(
				"Invalid Date Format",
				fmt.Sprintf("The date '%s' is not a valid calendar date. Please use YYYY-MM-DD format with a valid date (e.g., '2024-12-31').", input),
			)
			return
		}
		return // Valid date format and value
	}

	resp.Diagnostics.AddError(
		"Invalid Expiry Status Value",
		fmt.Sprintf("The value '%s' is not valid. Allowed values are 'enabled', 'locked', or a date in YYYY-MM-DD format (e.g., '2024-12-31').", input),
	)
}

// Custom validator for password attribute - basic validation only
type basicPasswordValidator struct{}

// Ensure basicPasswordValidator implements validator.String
var _ validator.String = basicPasswordValidator{}

// Description provides a human-readable description of the validator
func (v basicPasswordValidator) Description(ctx context.Context) string {
	return "Ensures the password meets basic requirements. Full policy validation is performed by F5OS device."
}

// MarkdownDescription provides a markdown description of the validator
func (v basicPasswordValidator) MarkdownDescription(ctx context.Context) string {
	return "Ensures the password meets basic requirements. Full policy validation is performed by F5OS device."
}

// ValidateString performs basic validation - let F5OS handle policy details
func (v basicPasswordValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return // Required field validation handled elsewhere
	}

	password := req.ConfigValue.ValueString()

	// Only basic validation - let F5OS handle the rest
	if len(password) == 0 {
		resp.Diagnostics.AddError(
			"Password Cannot Be Empty",
			"Password must not be empty.",
		)
		return
	}
}

// removeUserFromRole removes a user from a specific role using the roles endpoint
func (r *UserResource) removeUserFromRole(ctx context.Context, username, roleName string) error {
	uri := fmt.Sprintf("/openconfig-system:system/aaa/authentication/f5-system-aaa:roles/f5-system-aaa:role=%s/f5-system-aaa:config/f5-system-aaa:users=%s", roleName, username)

	tflog.Debug(ctx, "Removing User from Role", map[string]any{
		"username": username,
		"role":     roleName,
		"uri":      uri,
	})

	// Use DELETE request to remove the user from the role
	err := r.client.DeleteRequest(uri)
	if err != nil {
		errStr := err.Error()
		tflog.Debug(ctx, "Role Removal API Error", map[string]any{
			"error": errStr,
		})

		// Check if the user/role assignment doesn't exist (404 is OK for removal)
		if strings.Contains(errStr, "404") || strings.Contains(errStr, "not found") {
			tflog.Debug(ctx, "User was not assigned to role, removal not needed", map[string]any{
				"username": username,
				"role":     roleName,
			})
			return nil
		}

		return fmt.Errorf("API request failed: %w", err)
	}

	tflog.Debug(ctx, "User Role Removal Response", map[string]any{
		"username": username,
		"role":     roleName,
	})

	return nil
}
