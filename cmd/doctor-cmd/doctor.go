package doctorCmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/5uck1ess/raindrop-cli/internal/auth"
	"github.com/5uck1ess/raindrop-cli/internal/client"
	"github.com/spf13/cobra"
)

type apiUser struct {
	ID       int    `json:"_id"`
	Email    string `json:"email"`
	FullName string `json:"fullName"`
	Pro      bool   `json:"pro"`
}

type userResp struct {
	Result bool    `json:"result"`
	User   apiUser `json:"user"`
}

var doctorFlags struct {
	currentVersion string
}

const latestReleaseURL = "https://api.github.com/repos/5uck1ess/raindrop-cli/releases/latest"

var DoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Sanity check: auth, API rate-limit, CLI version, credential store",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("raindrop doctor")
		fmt.Println(strings.Repeat("─", 40))

		checkToken()
		checkAPI()
		checkKeychain()
		checkVersion()
	},
}

func checkToken() {
	if _, err := auth.Token(); err != nil {
		fmt.Printf("  token        ✗ %v\n", err)
		return
	}
	fmt.Printf("  token        ✓ RAINDROP_TOKEN is set\n")
}

func checkAPI() {
	c, err := client.New()
	if err != nil {
		fmt.Printf("  api          ✗ %v\n", err)
		return
	}
	var resp userResp
	if err := c.Do("GET", "/user", nil, &resp); err != nil {
		fmt.Printf("  api          ✗ %v\n", err)
		return
	}
	pro := ""
	if resp.User.Pro {
		pro = " (Pro)"
	}
	fmt.Printf("  api          ✓ authenticated as %s%s (id=%d)\n", resp.User.Email, pro, resp.User.ID)

	if rem := c.LastHeaders.Get("X-RateLimit-Remaining"); rem != "" {
		limit := c.LastHeaders.Get("X-RateLimit-Limit")
		reset := c.LastHeaders.Get("X-RateLimit-Reset")
		fmt.Printf("  rate-limit   ✓ %s / %s remaining (reset %s)\n", rem, limit, reset)
	} else {
		fmt.Printf("  rate-limit   · no X-RateLimit-* headers on last response\n")
	}
}

func checkKeychain() {
	if runtime.GOOS != "darwin" {
		return
	}
	out, err := exec.Command("security", "find-generic-password", "-s", "RAINDROP_TOKEN").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "RAINDROP_TOKEN") {
		fmt.Printf("  keychain     · no macOS Keychain entry under service 'RAINDROP_TOKEN'\n")
		return
	}
	fmt.Printf("  keychain     ✓ macOS Keychain entry found (service=RAINDROP_TOKEN)\n")
}

func checkVersion() {
	fmt.Printf("  version      · running %s\n", doctorFlags.currentVersion)
	httpc := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpc.Get(latestReleaseURL)
	if err != nil {
		fmt.Printf("  upstream     · could not reach GitHub: %v\n", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		fmt.Printf("  upstream     · GitHub returned HTTP %d\n", resp.StatusCode)
		return
	}
	body, _ := io.ReadAll(resp.Body)
	var data struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &data); err != nil {
		fmt.Printf("  upstream     · could not parse release JSON: %v\n", err)
		return
	}
	if doctorFlags.currentVersion == data.TagName || strings.HasPrefix(doctorFlags.currentVersion, data.TagName) {
		fmt.Printf("  upstream     ✓ up to date (%s)\n", data.TagName)
	} else {
		fmt.Printf("  upstream     ! latest is %s — upgrade available\n", data.TagName)
	}
}

// SetVersion lets the parent cmd inject the build's version string.
func SetVersion(v string) { doctorFlags.currentVersion = v }

func init() {
	_ = os.Getenv
}
