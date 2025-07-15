package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/autocrawlerHQ/browsergrid/internal/deployments"
)

var (
	apiURL  string
	apiKey  string
	verbose bool
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "browsergrid",
		Short: "BrowserGrid CLI for deployment management",
		Long:  `BrowserGrid CLI allows you to manage browser automation deployments`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "http://localhost:8765", "BrowserGrid API URL")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "BrowserGrid API key")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")

	// Bind flags to viper
	viper.BindPFlag("api-url", rootCmd.PersistentFlags().Lookup("api-url"))
	viper.BindPFlag("api-key", rootCmd.PersistentFlags().Lookup("api-key"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Environment variable support
	viper.SetEnvPrefix("BROWSERGRID")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Add commands
	rootCmd.AddCommand(initCmd())
	rootCmd.AddCommand(deployCmd())
	rootCmd.AddCommand(deploymentsCmd())
	rootCmd.AddCommand(logsCmd())
	rootCmd.AddCommand(scaleCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func initCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "init [directory]",
		Short: "Initialize a new deployment",
		Long:  `Initialize a new deployment in the specified directory (default: current directory)`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			return initDeployment(dir)
		},
	}

	return cmd
}

func deployCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "deploy [directory]",
		Short: "Deploy a package",
		Long:  `Deploy a package from the specified directory`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			return deployPackage(dir)
		},
	}

	return cmd
}

func deploymentsCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "deployments",
		Short: "Deployment management commands",
		Long:  `Commands for managing deployments`,
	}

	cmd.AddCommand(deploymentsListCmd())
	cmd.AddCommand(deploymentsShowCmd())
	cmd.AddCommand(deploymentsDeleteCmd())

	return cmd
}

func deploymentsListCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "list",
		Short: "List deployments",
		Long:  `List all deployments`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listDeployments()
		},
	}

	return cmd
}

func deploymentsShowCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "show [deployment-id]",
		Short: "Show deployment details",
		Long:  `Show detailed information about a deployment`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return showDeployment(args[0])
		},
	}

	return cmd
}

func deploymentsDeleteCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "delete [deployment-id]",
		Short: "Delete deployment",
		Long:  `Delete a deployment`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return deleteDeployment(args[0])
		},
	}

	return cmd
}

func logsCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "logs [run-id]",
		Short: "View logs",
		Long:  `View logs for a deployment run`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return viewLogs(args[0])
		},
	}

	return cmd
}

func scaleCmd() *cobra.Command {
	var instances int
	var cmd = &cobra.Command{
		Use:   "scale [deployment-id] --instances [count]",
		Short: "Scale a deployment",
		Long:  `Scale a deployment to the specified number of instances`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return scaleDeployment(args[0], instances)
		},
	}

	cmd.Flags().IntVar(&instances, "instances", 1, "Number of instances to scale to")
	cmd.MarkFlagRequired("instances")

	return cmd
}

// Implementation functions

func initDeployment(dir string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create browsergrid.json manifest
	manifestPath := filepath.Join(dir, "browsergrid.json")
	if _, err := os.Stat(manifestPath); err == nil {
		return fmt.Errorf("deployment already initialized (browsergrid.json exists)")
	}

	manifest := deployments.DeploymentManifest{
		Name:       filepath.Base(dir),
		Version:    "1.0.0",
		Runtime:    "node",
		EntryPoint: "index.js",
		Environment: map[string]string{
			"NODE_ENV": "production",
		},
		Config: deployments.DeploymentConfig{
			Concurrency:    1,
			MaxRetries:     3,
			TimeoutSeconds: 300,
			BrowserRequests: []deployments.BrowserRequest{
				{
					Browser:         "chrome",
					Version:         "latest",
					Headless:        true,
					OperatingSystem: "linux",
					Screen: map[string]interface{}{
						"width":  1920,
						"height": 1080,
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	// Create example script
	examplePath := filepath.Join(dir, "index.js")
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		exampleScript := `// BrowserGrid Deployment Example
import { chromium } from 'playwright';

async function main() {
  // Browser connection is injected via environment variables
  const browser = await chromium.connect({
    wsEndpoint: process.env.BROWSER_WS_ENDPOINT
  });
  
  const page = await browser.newPage();
  await page.goto('https://example.com');
  
  // Your automation logic here
  const title = await page.title();
  console.log('Page title:', title);
  
  // Return results (will be captured by BrowserGrid)
  return {
    title: title,
    url: page.url(),
    timestamp: new Date().toISOString()
  };
}

export default main;
`
		if err := os.WriteFile(examplePath, []byte(exampleScript), 0644); err != nil {
			return fmt.Errorf("failed to write example script: %w", err)
		}
	}

	// Create package.json for Node.js
	packagePath := filepath.Join(dir, "package.json")
	if _, err := os.Stat(packagePath); os.IsNotExist(err) {
		packageJSON := map[string]interface{}{
			"name":    filepath.Base(dir),
			"version": "1.0.0",
			"type":    "module",
			"main":    "index.js",
			"dependencies": map[string]string{
				"playwright": "^1.40.0",
			},
		}

		data, err := json.MarshalIndent(packageJSON, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal package.json: %w", err)
		}

		if err := os.WriteFile(packagePath, data, 0644); err != nil {
			return fmt.Errorf("failed to write package.json: %w", err)
		}
	}

	fmt.Printf("✓ Deployment initialized in %s\n", dir)
	fmt.Printf("  - browsergrid.json: Deployment configuration\n")
	fmt.Printf("  - index.js: Example automation script\n")
	fmt.Printf("  - package.json: Node.js dependencies\n")
	fmt.Printf("\nNext steps:\n")
	fmt.Printf("  1. Edit your automation script in index.js\n")
	fmt.Printf("  2. Configure deployment settings in browsergrid.json\n")
	fmt.Printf("  3. Run 'browsergrid deploy' to deploy your package\n")

	return nil
}

func deployPackage(dir string) error {
	// Load manifest
	manifest, err := deployments.LoadManifest(dir)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	fmt.Printf("Deploying %s v%s...\n", manifest.Name, manifest.Version)

	// TODO: Package the deployment (create ZIP file)
	// For now, we'll create a mock deployment

	deployment := deployments.CreateDeploymentRequest{
		Name:        manifest.Name,
		Version:     manifest.Version,
		Runtime:     deployments.Runtime(manifest.Runtime),
		PackageURL:  fmt.Sprintf("https://storage.example.com/deployments/%s-%s.zip", manifest.Name, manifest.Version),
		PackageHash: "mock-hash-" + manifest.Version,
		Config:      manifest.Config,
	}

	// Create deployment via API
	deploymentResp, err := apiRequest("POST", "/api/v1/deployments", deployment)
	if err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	fmt.Printf("✓ Deployment created successfully\n")
	fmt.Printf("  ID: %s\n", deploymentResp["id"])
	fmt.Printf("  Status: %s\n", deploymentResp["status"])

	return nil
}

func listDeployments() error {
	resp, err := apiRequest("GET", "/api/v1/deployments", nil)
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	deployments := resp["deployments"].([]interface{})
	if len(deployments) == 0 {
		fmt.Println("No deployments found")
		return nil
	}

	fmt.Printf("%-36s %-20s %-10s %-10s %s\n", "ID", "NAME", "VERSION", "RUNTIME", "STATUS")
	fmt.Println(strings.Repeat("-", 100))

	for _, d := range deployments {
		deployment := d.(map[string]interface{})
		fmt.Printf("%-36s %-20s %-10s %-10s %s\n",
			deployment["id"],
			deployment["name"],
			deployment["version"],
			deployment["runtime"],
			deployment["status"])
	}

	return nil
}

func showDeployment(id string) error {
	resp, err := apiRequest("GET", fmt.Sprintf("/api/v1/deployments/%s", id), nil)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to format deployment: %w", err)
	}

	fmt.Println(string(data))
	return nil
}

func deleteDeployment(id string) error {
	_, err := apiRequest("DELETE", fmt.Sprintf("/api/v1/deployments/%s", id), nil)
	if err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}

	fmt.Printf("✓ Deployment %s deleted successfully\n", id)
	return nil
}

func viewLogs(runID string) error {
	resp, err := apiRequest("GET", fmt.Sprintf("/api/v1/runs/%s/logs", runID), nil)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}

	fmt.Printf("Logs for run %s:\n", runID)
	fmt.Printf("Status: %s\n", resp["status"])
	fmt.Printf("Started: %s\n", resp["started_at"])
	if resp["completed_at"] != nil {
		fmt.Printf("Completed: %s\n", resp["completed_at"])
	}
	fmt.Println("\nOutput:")
	if resp["output"] != nil {
		output, _ := json.MarshalIndent(resp["output"], "", "  ")
		fmt.Println(string(output))
	}
	if resp["error"] != nil {
		fmt.Printf("\nError: %s\n", resp["error"])
	}

	return nil
}

func scaleDeployment(id string, instances int) error {
	// TODO: Implement scaling logic
	// This would create multiple deployment runs
	fmt.Printf("Scaling deployment %s to %d instances...\n", id, instances)

	for i := 0; i < instances; i++ {
		runReq := map[string]interface{}{
			"environment": map[string]string{
				"INSTANCE_ID": fmt.Sprintf("%d", i),
			},
		}

		_, err := apiRequest("POST", fmt.Sprintf("/api/v1/deployments/%s/runs", id), runReq)
		if err != nil {
			return fmt.Errorf("failed to create run instance %d: %w", i, err)
		}
	}

	fmt.Printf("✓ Created %d deployment runs\n", instances)
	return nil
}

func apiRequest(method, path string, body interface{}) (map[string]interface{}, error) {
	url := viper.GetString("api-url") + path
	key := viper.GetString("api-key")

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (%d): %s", resp.StatusCode, string(respData))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}
