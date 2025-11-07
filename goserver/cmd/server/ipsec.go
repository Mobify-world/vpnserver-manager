package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/go-chi/chi/v5"

// IPSecUser represents a simple user structure for API requests.
type IPSecUser struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

// ListIPSecUsers lists all IPSec users from chap-secrets.
func (app *application) ListIPSecUsers(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer cli.Close()

	execConfig := container.ExecOptions{
		Cmd:          []string{"cat", "/etc/ppp/chap-secrets"},
		AttachStdout: true,
		AttachStderr: true,
	}
	execID, err := cli.ContainerExecCreate(ctx, "ipsec-mobify-server", execConfig)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer resp.Close()

	scanner := bufio.NewScanner(resp.Reader)
	var users []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				users = append(users, fields[0])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.writeJSON(w, http.StatusOK, envolope{"users": users}, nil)
}

// AddIPSecUser adds a new user to the IPSec server.
func (app *application) AddIPSecUser(w http.ResponseWriter, r *http.Request) {
	var req IPSecUser
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Username == "" {
		app.badRequestResponse(w, r, fmt.Errorf("invalid request body"))
		return
	}

	if req.Password == "" {
		req.Password = "AutoGen123!"
	}

	cmd := []string{"bash", "-c", fmt.Sprintf("echo '%s l2tpd %s *' >> /etc/ppp/chap-secrets", req.Username, req.Password)}

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer cli.Close()

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Privileged:   true,
	}
	execID, err := cli.ContainerExecCreate(ctx, "ipsec-mobify-server", execConfig)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer resp.Close()

	app.writeJSON(w, http.StatusOK, envolope{
		"message": fmt.Sprintf("User %s added successfully", req.Username),
	}, nil)
}

// RemoveIPSecUser removes a user from chap-secrets.
func (app *application) RemoveIPSecUser(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if username == "" {
		app.badRequestResponse(w, r, fmt.Errorf("missing username"))
		return
	}

	cmd := []string{"bash", "-c", fmt.Sprintf("sed -i '/^%s /d' /etc/ppp/chap-secrets", username)}

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer cli.Close()

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Privileged:   true,
	}
	execID, err := cli.ContainerExecCreate(ctx, "ipsec-mobify-server", execConfig)
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	resp, err := cli.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}
	defer resp.Close()

	app.writeJSON(w, http.StatusOK, envolope{
		"message": fmt.Sprintf("User %s removed successfully", username),
	}, nil)
}
