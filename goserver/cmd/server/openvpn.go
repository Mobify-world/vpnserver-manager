package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

type OpenVPNProfile struct {
	Name string `json:"name"`
}

// ListOpenVPNProfiles lists available .ovpn profiles
func (app *application) ListOpenVPNProfiles(w http.ResponseWriter, r *http.Request) {
	files, err := filepath.Glob("/root/*.ovpn")
	if err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	var profiles []string
	for _, file := range files {
		profiles = append(profiles, filepath.Base(file))
	}

	app.writeJSON(w, http.StatusOK, envolope{"profiles": profiles}, nil)
}

// AddOpenVPNProfiles creates one or more OpenVPN client profiles
func (app *application) AddOpenVPNProfiles(w http.ResponseWriter, r *http.Request) {
	var req []OpenVPNProfile
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	for _, profile := range req {
		name := strings.TrimSpace(profile.Name)
		if name == "" {
			continue
		}

		cmd := exec.Command("bash", "openvpn.sh", "--addclient", name)
		cmd.Dir = "/root"
		if output, err := cmd.CombinedOutput(); err != nil {
			app.serverErrorResponse(w, r, fmt.Errorf("failed to create profile %s: %v", name, err))
			return
		} else {
			fmt.Printf("Created OpenVPN profile %s: %s\n", name, string(output))
		}
	}

	app.writeJSON(w, http.StatusOK, envolope{"message": "profiles created"}, nil)
}

// RemoveOpenVPNProfile removes an existing client profile
func (app *application) RemoveOpenVPNProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		app.badRequestResponse(w, r, fmt.Errorf("missing profile name"))
		return
	}

	profilePath := "/root/" + name + ".ovpn"
	if _, err := os.Stat(profilePath); os.IsNotExist(err) {
		app.badRequestResponse(w, r, fmt.Errorf("profile %s does not exist", name))
		return
	}

	if err := os.Remove(profilePath); err != nil {
		app.serverErrorResponse(w, r, err)
		return
	}

	app.writeJSON(w, http.StatusOK, envolope{"message": fmt.Sprintf("Profile %s removed", name)}, nil)
}
