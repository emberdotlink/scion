package cmd

import (
	"context"
	"fmt"

	"github.com/ptone/gswarm/pkg/runtime"
	"github.com/spf13/cobra"
)

// attachCmd represents the attach command
var attachCmd = &cobra.Command{
	Use:   "attach <agent>",
	Short: "Attach to an agent's interactive session",
	Long: `Attach to the interactive session of a running agent.
If the agent was started with tmux support, this will attach to the tmux session.`, 
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentName := args[0]
		rt := runtime.GetRuntime()
		
		fmt.Printf("Attaching to agent '%s'...\n", agentName)
		return rt.Attach(context.Background(), agentName)
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
}

