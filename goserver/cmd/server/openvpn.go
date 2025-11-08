package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// OpenVPN request/response structs
type AddOpenVPNProfileRequest struct {
	ClientName string `json:"client_name"`
}

type DeleteOpenVPNProfileRequest struct {
	ClientName string `json:"client_name"`
}

type OpenVPNProfile struct {
	ClientName string `json:"client_name"`
	FilePath   string `json:"file_path"`
	FileSize   int64  `json:"file_size"`
	Content    string `json:"content,omitempty"` // Optional: include file content
}

type OpenVPNProfilesResponse struct {
	Profiles []OpenVPNProfile `json:"profiles"`
	Count    int              `json:"count"`
}

// 1. Add OpenVPN Profile
func (app *application) AddOpenVPNProfileHandler(w http.ResponseWriter, r *http.Request) {
	var req AddOpenVPNProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.badRequestResponse(w, r, errors.New("invalid request body"))
		return
	}

	if req.ClientName == "" {
		app.badRequestResponse(w, r, errors.New("client_name is required"))
		return
	}

	// Validate client name (alphanumeric and hyphen/underscore only)
	if !isValidClientName(req.ClientName) {
		app.badRequestResponse(w, r, errors.New("invalid client_name: only alphanumeric, hyphen, and underscore allowed"))
		return
	}

	// Check if profile already exists
	profilePath := filepath.Join("/root", req.ClientName+".ovpn")
	if _, err := os.Stat(profilePath); err == nil {
		app.badRequestResponse(w, r, fmt.Errorf("profile '%s' already exists", req.ClientName))
		return
	}

	// Execute openvpn.sh script to add client
	cmd := []string{"bash", "/root/openvpn.sh", "--addclient", req.ClientName}
	result, err := app.dockerExecHost(cmd)
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to create OpenVPN profile: %v", err))
		return
	}

	// Wait a moment for file to be written
	time.Sleep(1 * time.Second)

	// Verify profile was created
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		app.serverErrorResponse(w, r, errors.New("profile creation failed: file not found"))
		return
	}

	// Read the profile content
	content, err := os.ReadFile(profilePath)
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to read profile: %v", err))
		return
	}

	profile := OpenVPNProfile{
		ClientName: req.ClientName,
		FilePath:   profilePath,
		FileSize:   int64(len(content)),
		Content:    string(content),
	}

	err = app.writeJSON(w, http.StatusOK, envolope{
		"message": fmt.Sprintf("OpenVPN profile '%s' created successfully", req.ClientName),
		"profile": profile,
		"output":  result,
	}, nil)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

// 2. Delete OpenVPN Profile
func (app *application) DeleteOpenVPNProfileHandler(w http.ResponseWriter, r *http.Request) {
	var req DeleteOpenVPNProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.badRequestResponse(w, r, errors.New("invalid request body"))
		return
	}

	if req.ClientName == "" {
		app.badRequestResponse(w, r, errors.New("client_name is required"))
		return
	}

	// Don't allow deleting the default client
	if req.ClientName == "client" {
		app.badRequestResponse(w, r, errors.New("cannot delete default 'client' profile"))
		return
	}

	// Check if profile exists
	profilePath := filepath.Join("/root", req.ClientName+".ovpn")
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		app.badRequestResponse(w, r, fmt.Errorf("profile '%s' does not exist", req.ClientName))
		return
	}

	// Revoke certificate using openvpn.sh
	cmd := []string{"bash", "/root/openvpn.sh", "--revoke", req.ClientName}
	result, err := app.dockerExecHost(cmd)
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to revoke certificate: %v", err))
		return
	}

	// Delete the .ovpn file
	if err := os.Remove(profilePath); err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to delete profile file: %v", err))
		return
	}

	err = app.writeJSON(w, http.StatusOK, envolope{
		"message": fmt.Sprintf("OpenVPN profile '%s' deleted successfully", req.ClientName),
		"output":  result,
	}, nil)

	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

// 3. Get All OpenVPN Profiles
func (app *application) GetOpenVPNProfilesHandler(w http.ResponseWriter, r *http.Request) {
	// Query parameter to include file content (default: false)
	includeContent := r.URL.Query().Get("include_content") == "true"

	// Read all .ovpn files from /root directory
	files, err := filepath.Glob("/root/*.ovpn")
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to list profiles: %v", err))
		return
	}

	profiles := make([]OpenVPNProfile, 0)

	for _, filePath := range files {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			continue // Skip files we can't read
		}

		clientName := strings.TrimSuffix(filepath.Base(filePath), ".ovpn")

		profile := OpenVPNProfile{
			ClientName: clientName,
			FilePath:   filePath,
			FileSize:   fileInfo.Size(),
		}

		// Optionally include file content
		if includeContent {
			content, err := os.ReadFile(filePath)
			if err == nil {
				profile.Content = string(content)
			}
		}

		profiles = append(profiles, profile)
	}

	response := OpenVPNProfilesResponse{
		Profiles: profiles,
		Count:    len(profiles),
	}

	err = app.writeJSON(w, http.StatusOK, envolope{"data": response}, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
}

// Helper function to execute commands on the host using os/exec
// Helper function to execute commands on the HOST using Docker
func (app *application) dockerExecHost(cmd []string) (string, error) {
	// Execute command on the host by running a new container with host PID namespace
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return "", err
	}
	defer cli.Close()

	// Create a container that runs in host PID namespace and executes the command
	containerConfig := &container.Config{
		Image: "ubuntu:latest", // Use Ubuntu to run the script
		Cmd:   cmd,
		Tty:   false,
	}

	hostConfig := &container.HostConfig{
		AutoRemove: true,
		Binds: []string{
			"/root:/root",
			"/etc/openvpn:/etc/openvpn",
		},
		PidMode:    "host",
		Privileged: true,
	}

	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return "", fmt.Errorf("failed to create container: %v", err)
	}

	if err := cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %v", err)
	}

	// Wait for container to finish
	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return "", err
		}
	case <-statusCh:
	}

	// Get logs
	out, err := cli.ContainerLogs(ctx, resp.ID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		return "", err
	}
	defer out.Close()

	var buf bytes.Buffer
	_, err = stdcopy.StdCopy(&buf, &buf, out)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// Helper function to validate client names
func isValidClientName(name string) bool {
	if name == "" || len(name) > 64 {
		return false
	}
	for _, char := range name {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return false
		}
	}
	return true
}
