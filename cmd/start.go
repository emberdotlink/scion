package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ptone/gswarm/pkg/config"
	"github.com/ptone/gswarm/pkg/runtime"
	"github.com/ptone/gswarm/pkg/util"
	"github.com/spf13/cobra"
)

var (
	agentName    string
	templateName string
	agentImage   string
	noAuth       bool
	attach       bool
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start <task>",
	Short: "Launch a new gswarm agent",
	Long: `Provision and launch a new isolated Gemini agent to perform a specific task.
The agent will be created from a template and run in a detached container.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		task := args[0]

		if agentName == "" {
			// Generate a unique name if not provided
			agentName = fmt.Sprintf("agent-%d", os.Getpid())
		}

		fmt.Printf("Starting agent '%s' for task: %s\n", agentName, task)

		// 1. Prepare agent directories
		agentsDir, err := config.GetProjectAgentsDir()
		if err != nil {
			return err
		}
		agentDir := filepath.Join(agentsDir, agentName)
		agentHome := filepath.Join(agentDir, "home")
		agentWorkspace := filepath.Join(agentDir, "workspace")

		if err := os.MkdirAll(agentHome, 0755); err != nil {
			return fmt.Errorf("failed to create agent home: %w", err)
		}
		if err := os.MkdirAll(agentWorkspace, 0755); err != nil {
			return fmt.Errorf("failed to create agent workspace: %w", err)
		}

		// 2. Load and copy templates
		chain, err := config.GetTemplateChain(templateName)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}

		// Track image from templates
		resolvedImage := ""

		for _, tpl := range chain {
			fmt.Printf("Applying template: %s\n", tpl.Name)
			if err := util.CopyDir(tpl.Path, agentHome); err != nil {
				return fmt.Errorf("failed to copy template %s: %w", tpl.Name, err)
			}

			// Load gswarm.json from this template to see if it specifies an image
			tplCfg, err := tpl.LoadConfig()
			if err == nil && tplCfg.Image != "" {
				resolvedImage = tplCfg.Image
			}
		}

		// Flag takes ultimate precedence
		if agentImage != "" {
			resolvedImage = agentImage
		}
		if resolvedImage == "" {
			resolvedImage = "gemini-cli-sandbox"
		}

		// 3. Propagate credentials
		var auth config.AuthConfig
		if !noAuth {
			// Load agent settings from the newly prepared home directory
			agentSettingsPath := filepath.Join(agentHome, ".gemini", "settings.json")
			agentSettings, _ := config.LoadGeminiSettings(agentSettingsPath)
			auth = config.DiscoverAuth(agentSettings)
		}

		// 4. Launch container
		rt := runtime.GetRuntime()

		// Determine detached mode and tmux from templates (last one wins)
		detached := true
		useTmux := false
		for _, tpl := range chain {
			tplCfg, err := tpl.LoadConfig()
			if err == nil {
				detached = tplCfg.IsDetached()
				if tplCfg.UseTmux {
					useTmux = true
				}
			}
		}

		if useTmux {
			tmuxImage := resolvedImage
			if !strings.Contains(tmuxImage, ":") {
				tmuxImage = tmuxImage + ":tmux"
			} else {
				parts := strings.SplitN(resolvedImage, ":", 2)
				tmuxImage = parts[0] + ":tmux"
			}

			exists, err := rt.ImageExists(context.Background(), tmuxImage)
			if err != nil || !exists {
				return fmt.Errorf("tmux support requested but image '%s' not found. Please ensure the image has a :tmux tag.", tmuxImage)
			}
			resolvedImage = tmuxImage
		}

		// If user requested attach, we must run detached first then attach
		// OR we just run it interactive.
		// Let's say if attach is requested, we run detached then attach.
		// If detached is false in config, we run interactive and ignore attach flag.
		
		runCfg := runtime.RunConfig{
			Name:      agentName,
			Image:     resolvedImage,
			HomeDir:   agentHome,
			Workspace: agentWorkspace,
			Auth:      auth,
			Detached:  detached || attach,
			UseTmux:   useTmux,
			Env: []string{
				fmt.Sprintf("GEMINI_INITIAL_PROMPT=%s", task),
				fmt.Sprintf("GEMINI_AGENT_NAME=%s", agentName),
			},
			Labels: map[string]string{
				"gswarm.agent": "true",
				"gswarm.name":  agentName,
			},
		}

		id, err := rt.Run(context.Background(), runCfg)
		if err != nil {
			return fmt.Errorf("failed to launch container: %w", err)
		}

		if runCfg.Detached {
			fmt.Printf("Agent '%s' launched successfully (ID: %s)\n", agentName, id)
		}

		if attach && runCfg.Detached {
			fmt.Printf("Attaching to agent '%s'...\n", agentName)
			return rt.Attach(context.Background(), id)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringVarP(&agentName, "name", "n", "", "Name of the agent")
	startCmd.Flags().StringVarP(&templateName, "type", "t", "default", "Template to use")
	startCmd.Flags().StringVarP(&agentImage, "image", "i", "", "Container image to use (overrides template)")
	startCmd.Flags().BoolVar(&noAuth, "no-auth", false, "Disable authentication propagation")
	startCmd.Flags().BoolVarP(&attach, "attach", "a", false, "Attach to the agent TTY after starting")
}
			
