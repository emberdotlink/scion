package runtime

import (
	"context"

	"github.com/ptone/scion-agent/pkg/config"
)

type AgentInfo struct {
	ID          string
	Name        string
	Template    string
	Grove       string
	GrovePath   string
	Labels      map[string]string
	Status      string // Container status
	AgentStatus string // Scion agent high-level status
	Image       string
}

type RunConfig struct {
	Name         string
	Template     string
	UnixUsername string
	Image        string
	HomeDir      string
	Workspace    string
	RepoRoot     string
	Env          []string
	Labels       map[string]string
	Auth         config.AuthConfig
	UseTmux      bool
	Model        string
	Task         string
	Resume       bool
}

type Runtime interface {
	Run(ctx context.Context, config RunConfig) (string, error)
	Stop(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, labelFilter map[string]string) ([]AgentInfo, error)
	GetLogs(ctx context.Context, id string) (string, error)
	Attach(ctx context.Context, id string) error
	ImageExists(ctx context.Context, image string) (bool, error)
}
