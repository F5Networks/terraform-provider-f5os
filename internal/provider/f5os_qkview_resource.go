package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	f5ossdk "gitswarm.f5net.com/terraform-providers/f5osclient"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &QkviewResource{}

func NewQkviewResource() resource.Resource {
	return &QkviewResource{}
}

// QkviewResource defines the resource implementation.
type QkviewResource struct {
	client *f5ossdk.F5os
}

type QkviewResourceModel struct {
	Filename      types.String `tfsdk:"filename"`
	Timeout       types.Int64  `tfsdk:"timeout"`
	MaxFileSize   types.Int64  `tfsdk:"max_file_size"`
	MaxCoreSize   types.Int64  `tfsdk:"max_core_size"`
	ExcludeCores  types.Bool   `tfsdk:"exclude_cores"`
	Id            types.String `tfsdk:"id"`
	GeneratedFile types.String `tfsdk:"generated_file"`
	Status        types.String `tfsdk:"status"`
}

// QkviewCaptureRequest represents the API request structure for qkview capture
type QkviewCaptureRequest struct {
	Filename     string `json:"filename"`
	Timeout      int64  `json:"timeout,omitempty"`
	MaxFileSize  int64  `json:"maxfilesize,omitempty"`
	MaxCoreSize  int64  `json:"maxcoresize,omitempty"`
	ExcludeCores bool   `json:"exclude-cores,omitempty"`
}

// QkviewStatusResponse represents the API response structure for qkview status
type QkviewStatusResponse struct {
	Output struct {
		Result string `json:"result"`
	} `json:"f5-system-diagnostics-qkview:output"`
}

// QkviewListResponse represents the API response structure for qkview list
type QkviewListResponse struct {
	Output struct {
		Result string `json:"result"`
	} `json:"f5-system-diagnostics-qkview:output"`
}

// QkviewStatus represents the parsed status result
type QkviewStatus struct {
	Percent int    `json:"Percent"`
	Status  string `json:"Status"`
	Message string `json:"Message"`
}

// QkviewList represents the parsed list result
type QkviewList struct {
	Qkviews []struct {
		Filename string `json:"Filename"`
	} `json:"Qkviews"`
}

func (r *QkviewResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_qkview"
}

func (r *QkviewResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Resource used to generate and manage qkview diagnostic files on F5OS devices. Qkview files contain system information and logs for troubleshooting purposes.",

		Attributes: map[string]schema.Attribute{
			"filename": schema.StringAttribute{
				MarkdownDescription: "Name of the qkview file to generate (without .tar extension).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"timeout": schema.Int64Attribute{
				MarkdownDescription: "Timeout value in seconds for qkview generation. Default is 0 (no timeout).",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(0),
			},
			"max_file_size": schema.Int64Attribute{
				MarkdownDescription: "Maximum file size in megabytes. Must be between 2-1000. Default is 500.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(500),
			},
			"max_core_size": schema.Int64Attribute{
				MarkdownDescription: "Maximum core size in megabytes. Must be between 2-1000. Default is 25.",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(25),
			},
			"exclude_cores": schema.BoolAttribute{
				MarkdownDescription: "Specifies whether to exclude cores from the qkview. Default is false.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Unique identifier for the resource.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"generated_file": schema.StringAttribute{
				MarkdownDescription: "Full filename of the generated qkview file (including path and extension).",
				Computed:            true,
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current status of the qkview generation.",
				Computed:            true,
			},
		},
	}
}

func (r *QkviewResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client, resp.Diagnostics = toF5osProvider(req.ProviderData)
}

func (r *QkviewResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *QkviewResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Validate parameters
	if err := r.validateQkviewParams(data); err != nil {
		resp.Diagnostics.AddError("Validation Error", err.Error())
		return
	}

	filename := data.Filename.ValueString()
	tflog.Info(ctx, fmt.Sprintf("Creating qkview: %s", filename))

	// Check if qkview already exists
	exists, existingFile, err := r.qkviewExists(ctx, filename)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Failed to check if qkview exists: %s", err))
		return
	}

	if exists {
		resp.Diagnostics.AddError("Resource Already Exists", fmt.Sprintf("Qkview file %s already exists as %s", filename, existingFile))
		return
	}

	// Create qkview
	err = r.createQkview(ctx, data)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Qkview creation failed: %s", err))
		return
	}

	// Wait for completion and get final status
	generatedFile, err := r.waitForCompletion(ctx, filename)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Qkview generation failed: %s", err))
		return
	}

	data.Id = types.StringValue(filename)
	data.GeneratedFile = types.StringValue(generatedFile)
	data.Status = types.StringValue("complete")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *QkviewResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *QkviewResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	filename := data.Filename.ValueString()

	// Check if qkview still exists
	exists, existingFile, err := r.qkviewExists(ctx, filename)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Failed to check qkview status: %s", err))
		return
	}

	if !exists {
		// Qkview was deleted outside of Terraform
		resp.State.RemoveResource(ctx)
		return
	}

	data.GeneratedFile = types.StringValue(existingFile)
	data.Status = types.StringValue("complete")

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *QkviewResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *QkviewResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Since filename requires replacement, this should not be called
	// But if other parameters change, we need to recreate the qkview
	resp.Diagnostics.AddError("Update Not Supported", "Qkview parameters cannot be updated. Changes require resource replacement.")
}

func (r *QkviewResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *QkviewResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	filename := data.Filename.ValueString()
	tflog.Info(ctx, fmt.Sprintf("Deleting qkview: %s", filename))

	// Get the actual filename to delete
	_, existingFile, err := r.qkviewExists(ctx, filename)
	if err != nil {
		tflog.Warn(ctx, fmt.Sprintf("Failed to check qkview existence during delete: %s", err))
		// Continue with deletion attempt anyway
	}

	if existingFile == "" {
		// Try with the original filename
		existingFile = filename
		if !strings.HasSuffix(existingFile, ".tar") {
			existingFile = existingFile + ".tar"
		}
	}

	// Delete qkview
	err = r.deleteQkview(ctx, existingFile)
	if err != nil {
		resp.Diagnostics.AddError("F5OS Client Error", fmt.Sprintf("Failed to delete qkview: %s", err))
		return
	}

	tflog.Info(ctx, fmt.Sprintf("Successfully deleted qkview: %s", existingFile))
}

// validateQkviewParams validates the qkview parameters
func (r *QkviewResource) validateQkviewParams(data *QkviewResourceModel) error {
	maxFileSize := data.MaxFileSize.ValueInt64()
	maxCoreSize := data.MaxCoreSize.ValueInt64()

	if maxFileSize < 2 || maxFileSize > 1000 {
		return fmt.Errorf("max_file_size must be between 2-1000, got %d", maxFileSize)
	}

	if maxCoreSize < 2 || maxCoreSize > 1000 {
		return fmt.Errorf("max_core_size must be between 2-1000, got %d", maxCoreSize)
	}

	return nil
}

// qkviewExists checks if a qkview file exists on the F5OS device
func (r *QkviewResource) qkviewExists(ctx context.Context, filename string) (bool, string, error) {
	uri := "/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/list"

	response, err := r.client.PostRequest(uri, nil)
	if err != nil {
		return false, "", fmt.Errorf("failed to list qkviews: %w", err)
	}

	var listResponse QkviewListResponse
	if err := json.Unmarshal(response, &listResponse); err != nil {
		return false, "", fmt.Errorf("failed to parse list response: %w", err)
	}

	var qkviewList QkviewList
	if err := json.Unmarshal([]byte(listResponse.Output.Result), &qkviewList); err != nil {
		return false, "", fmt.Errorf("failed to parse qkview list: %w", err)
	}

	if qkviewList.Qkviews == nil {
		return false, "", nil
	}

	// Check if our filename exists (with or without .tar extension)
	targetBase := filename

	// Normalize target filename - remove .tar if present to get base name
	targetBase = strings.TrimSuffix(targetBase, ".tar")

	tflog.Debug(ctx, fmt.Sprintf("Looking for qkview with base name: %s", targetBase))

	for _, item := range qkviewList.Qkviews {
		// Handle filenames that may have path prefixes (like "slot:filename" or "controller-1:filename")
		existingFilename := item.Filename
		existingBase := existingFilename

		// Remove any prefix (slot:, controller-1:, etc.)
		if strings.Contains(existingBase, ":") {
			parts := strings.Split(existingBase, ":")
			if len(parts) > 1 {
				existingBase = parts[len(parts)-1] // Take the last part after ":"
			}
		}

		// Remove compound suffixes in the correct order
		// Handle .tar.timedout, .tar.canceled, etc.
		if strings.HasSuffix(existingBase, ".tar.timedout") {
			existingBase = strings.TrimSuffix(existingBase, ".tar.timedout")
		} else if strings.HasSuffix(existingBase, ".tar.canceled") {
			existingBase = strings.TrimSuffix(existingBase, ".tar.canceled")
		} else if strings.HasSuffix(existingBase, ".timedout") {
			existingBase = strings.TrimSuffix(existingBase, ".timedout")
		} else if strings.HasSuffix(existingBase, ".canceled") {
			existingBase = strings.TrimSuffix(existingBase, ".canceled")
		} else if strings.HasSuffix(existingBase, ".tar") {
			existingBase = strings.TrimSuffix(existingBase, ".tar")
		}

		tflog.Debug(ctx, fmt.Sprintf("Comparing target '%s' with existing '%s' (original: %s)", targetBase, existingBase, item.Filename))

		// Match based on base filename (without extensions and prefixes)
		if targetBase == existingBase {
			tflog.Info(ctx, fmt.Sprintf("Found matching qkview: %s matches %s", targetBase, item.Filename))
			return true, item.Filename, nil
		}
	}

	return false, "", nil
}

// createQkview initiates qkview generation
func (r *QkviewResource) createQkview(ctx context.Context, data *QkviewResourceModel) error {
	uri := "/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/capture"

	captureRequest := QkviewCaptureRequest{
		Filename:     data.Filename.ValueString(),
		Timeout:      data.Timeout.ValueInt64(),
		MaxFileSize:  data.MaxFileSize.ValueInt64(),
		MaxCoreSize:  data.MaxCoreSize.ValueInt64(),
		ExcludeCores: data.ExcludeCores.ValueBool(),
	}

	requestBody, err := json.Marshal(captureRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal capture request: %w", err)
	}

	_, err = r.client.PostRequest(uri, requestBody)
	if err != nil {
		return fmt.Errorf("failed to initiate qkview capture: %w", err)
	}

	tflog.Debug(ctx, "Qkview capture initiated successfully")
	return nil
}

// waitForCompletion polls the qkview status until completion
func (r *QkviewResource) waitForCompletion(ctx context.Context, filename string) (string, error) {
	uri := "/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/status"

	statuses := []string{"collating", "collecting"}
	messages := []string{"Collecting Data", "Collating data"}

	// Poll for up to 30 minutes (180 attempts * 10 seconds)
	maxAttempts := 180
	pollInterval := 10 * time.Second

	for attempt := 0; attempt < maxAttempts; attempt++ {
		response, err := r.client.PostRequest(uri, nil)
		if err != nil {
			tflog.Error(ctx, fmt.Sprintf("Failed to check qkview status on attempt %d: %v", attempt+1, err))
			// Don't fail immediately, retry a few times in case of transient network issues
			if attempt >= 3 {
				return "", fmt.Errorf("failed to check qkview status after %d attempts: %w", attempt+1, err)
			}
			time.Sleep(pollInterval)
			continue
		}

		tflog.Debug(ctx, fmt.Sprintf("Status check attempt %d: received response", attempt+1))

		var statusResponse QkviewStatusResponse
		if err := json.Unmarshal(response, &statusResponse); err != nil {
			tflog.Error(ctx, fmt.Sprintf("Failed to parse status response on attempt %d: %v", attempt+1, err))
			tflog.Debug(ctx, fmt.Sprintf("Raw response: %s", string(response)))
			// Don't fail immediately for parse errors either
			if attempt >= 3 {
				return "", fmt.Errorf("failed to parse status response after %d attempts: %w", attempt+1, err)
			}
			time.Sleep(pollInterval)
			continue
		}

		var status QkviewStatus
		if err := json.Unmarshal([]byte(statusResponse.Output.Result), &status); err != nil {
			tflog.Error(ctx, fmt.Sprintf("Failed to parse status result on attempt %d: %v", attempt+1, err))
			tflog.Debug(ctx, fmt.Sprintf("Raw result: %s", statusResponse.Output.Result))
			// Don't fail immediately for parse errors either
			if attempt >= 3 {
				return "", fmt.Errorf("failed to parse status result after %d attempts: %w", attempt+1, err)
			}
			time.Sleep(pollInterval)
			continue
		}

		tflog.Debug(ctx, fmt.Sprintf("Qkview status: %d%% - %s - %s", status.Percent, status.Status, status.Message))

		// Check if completed successfully (including timeout scenarios with partial files)
		if status.Percent == 100 || status.Status == "complete" ||
			strings.Contains(strings.ToLower(status.Message), "completed") ||
			strings.Contains(strings.ToLower(status.Status), "complete") ||
			status.Status == "time-out" || strings.Contains(strings.ToLower(status.Status), "timeout") ||
			strings.Contains(strings.ToLower(status.Message), "timed out") ||
			strings.Contains(strings.ToLower(status.Message), "partial qkview saved") {

			tflog.Info(ctx, fmt.Sprintf("Qkview generation finished (status: %s), checking for generated file", status.Status))

			// Get the actual generated filename
			exists, generatedFile, err := r.qkviewExists(ctx, filename)
			if err != nil {
				tflog.Warn(ctx, fmt.Sprintf("Failed to check if qkview exists: %v", err))
				// For timeout scenarios, wait a bit longer for file system to catch up
				if status.Status == "time-out" || strings.Contains(strings.ToLower(status.Message), "timed out") {
					time.Sleep(5 * time.Second) // Wait 5 seconds for file to appear
					exists, generatedFile, err = r.qkviewExists(ctx, filename)
					if err == nil && exists {
						tflog.Info(ctx, fmt.Sprintf("Timeout qkview file found after delay: %s", generatedFile))
						return generatedFile, nil
					}
				}
				// Continue polling in case the file isn't ready yet
				time.Sleep(pollInterval)
				continue
			}
			if exists {
				tflog.Info(ctx, fmt.Sprintf("Qkview file found: %s", generatedFile))
				return generatedFile, nil
			} else {
				// For timeout scenarios, don't keep polling forever
				if status.Status == "time-out" || strings.Contains(strings.ToLower(status.Message), "timed out") {
					tflog.Warn(ctx, "Qkview timed out on device but no file found - treating as success with partial file")
					// Return a reasonable filename even if we can't verify it exists
					return fmt.Sprintf("%s.timedout", filename), nil
				}
				tflog.Debug(ctx, "Qkview file not found yet, continuing to poll")
				time.Sleep(pollInterval)
				continue
			}
		}

		// Check if still in progress
		if status.Percent < 100 && (contains(statuses, status.Status) || contains(messages, status.Message) ||
			strings.Contains(strings.ToLower(status.Status), "collect") ||
			strings.Contains(strings.ToLower(status.Message), "collect")) {
			time.Sleep(pollInterval)
			continue
		}

		// If we get here and percent is less than 100, something might be wrong but continue polling
		if status.Percent < 100 {
			tflog.Warn(ctx, fmt.Sprintf("Unknown qkview status: %d%% - %s - %s, continuing to poll", status.Percent, status.Status, status.Message))
			time.Sleep(pollInterval)
			continue
		}

		// If we get here, something went wrong
		return "", fmt.Errorf("qkview generation failed: %s - %s", status.Status, status.Message)
	}

	return "", fmt.Errorf("qkview generation timed out after %d attempts", maxAttempts)
}

// deleteQkview deletes a qkview file from the F5OS device
func (r *QkviewResource) deleteQkview(ctx context.Context, filename string) error {
	uri := "/openconfig-system:system/f5-system-diagnostics-qkview:diagnostics/qkview/delete"

	deleteRequest := map[string]string{
		"filename": filename,
	}

	requestBody, err := json.Marshal(deleteRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal delete request: %w", err)
	}

	response, err := r.client.PostRequest(uri, requestBody)
	if err != nil {
		return fmt.Errorf("failed to delete qkview: %w", err)
	}

	// Check if the response indicates an error
	if strings.Contains(string(response), "Error deleting") {
		return fmt.Errorf("delete operation failed: %s", string(response))
	}

	return nil
}

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
