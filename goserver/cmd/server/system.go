package main

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

func (app *application) RestartAllServices(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	// Restart IPSec Docker container
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithVersion("1.47"))
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to init docker client: %v", err))
		return
	}
	defer cli.Close()

	timeout := 5
	if err := cli.ContainerRestart(ctx, "ipsec-mobify-server", container.StopOptions{Timeout: &timeout}); err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to restart ipsec container: %v", err))
		return
	}

	// Restart OpenVPN service
	cmd := exec.Command("systemctl", "restart", "openvpn-server@server.service")
	output, err := cmd.CombinedOutput()
	if err != nil {
		app.serverErrorResponse(w, r, fmt.Errorf("failed to restart openvpn service: %v (%s)", err, string(output)))
		return
	}

	app.writeJSON(w, http.StatusOK, envolope{
		"message": "IPSec and OpenVPN services restarted successfully",
	}, nil)
}
