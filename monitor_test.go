package ecsazrlc

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// TestNewMonitor vérifie la création d'un nouveau moniteur
func TestNewMonitor(t *testing.T) {
	monitor, err := NewMonitor()
	if err != nil {
		t.Skipf("Skipping test: Docker not available - %v", err)
	}
	defer monitor.Stop()

	if monitor == nil {
		t.Fatal("Expected monitor to be created, got nil")
	}

	if monitor.dockerClient == nil {
		t.Error("Expected dockerClient to be initialized")
	}

	if monitor.activityChan == nil {
		t.Error("Expected activityChan to be initialized")
	}
}

// TestIsAzureAgentContainer vérifie la détection des conteneurs Azure Agent
func TestIsAzureAgentContainer(t *testing.T) {
	monitor := &Monitor{}

	tests := []struct {
		name     string
		container types.ContainerJSON
		expected bool
	}{
		{
			name: "Azure agent by image name",
			container: types.ContainerJSON{
				Config: &container.Config{
					Image: "myregistry/azure-agent:latest",
				},
			},
			expected: true,
		},
		{
			name: "AZP agent by image name",
			container: types.ContainerJSON{
				Config: &container.Config{
					Image: "azp-agent:v1",
				},
			},
			expected: true,
		},
		{
			name: "VSTS agent by image name",
			container: types.ContainerJSON{
				Config: &container.Config{
					Image: "vsts-agent:latest",
				},
			},
			expected: true,
		},
		{
			name: "Azure agent by environment variable",
			container: types.ContainerJSON{
				Config: &container.Config{
					Image: "some-image:latest",
					Env: []string{
						"AZP_URL=https://dev.azure.com",
						"AZP_TOKEN=secret",
					},
				},
			},
			expected: true,
		},
		{
			name: "Azure agent by label",
			container: types.ContainerJSON{
				Config: &container.Config{
					Image: "some-image:latest",
					Labels: map[string]string{
						"app":  "azure-agent",
						"type": "devops",
					},
				},
			},
			expected: true,
		},
		{
			name: "Not an Azure agent",
			container: types.ContainerJSON{
				Config: &container.Config{
					Image: "nginx:latest",
					Env:   []string{"PORT=8080"},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := monitor.IsAzureAgentContainer(tt.container)
			if result != tt.expected {
				t.Errorf("IsAzureAgentContainer() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestActivityEvent vérifie la structure ActivityEvent
func TestActivityEvent(t *testing.T) {
	event := ActivityEvent{
		ContainerID:   "abc123",
		ContainerName: "test-agent",
		ImageName:     "azure-agent:latest",
		Action:        "start",
		Timestamp:     time.Now(),
		IsAzureAgent:  true,
	}

	if event.ContainerID == "" {
		t.Error("Expected ContainerID to be set")
	}
	if event.Action != "start" {
		t.Errorf("Expected Action to be 'start', got %s", event.Action)
	}
	if !event.IsAzureAgent {
		t.Error("Expected IsAzureAgent to be true")
	}
}

// TestGetActivityChannel vérifie que le canal d'activité fonctionne
func TestGetActivityChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	monitor := &Monitor{
		ctx:          ctx,
		cancel:       cancel,
		activityChan: make(chan ActivityEvent, 10),
	}

	ch := monitor.GetActivityChannel()
	if ch == nil {
		t.Fatal("Expected activity channel to be non-nil")
	}

	// Test d'envoi d'événement
	testEvent := ActivityEvent{
		ContainerID:  "test123",
		Action:       "start",
		IsAzureAgent: true,
		Timestamp:    time.Now(),
	}

	go func() {
		monitor.activityChan <- testEvent
	}()

	select {
	case event := <-ch:
		if event.ContainerID != testEvent.ContainerID {
			t.Errorf("Expected ContainerID %s, got %s", testEvent.ContainerID, event.ContainerID)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for event")
	}
}

// TestHasActiveAgents vérifie la détection d'agents actifs
func TestHasActiveAgents(t *testing.T) {
	monitor, err := NewMonitor()
	if err != nil {
		t.Skipf("Skipping test: Docker not available - %v", err)
	}
	defer monitor.Stop()

	// Test avec Docker disponible
	hasAgents, err := monitor.HasActiveAgents()
	if err != nil {
		// Docker peut ne pas être accessible, ce n'est pas une erreur de test
		t.Skipf("Skipping test: Docker not accessible - %v", err)
	}

	// Nous ne savons pas s'il y a des agents, mais la fonction ne doit pas échouer
	t.Logf("Has active agents: %v", hasAgents)
}

// TestMonitorStop vérifie l'arrêt propre du moniteur
func TestMonitorStop(t *testing.T) {
	monitor, err := NewMonitor()
	if err != nil {
		t.Skipf("Skipping test: Docker not available - %v", err)
	}

	// Vérifier que le canal est ouvert
	select {
	case <-monitor.activityChan:
		// Canal vide, c'est normal
	default:
		// Canal disponible pour écriture, c'est normal
	}

	monitor.Stop()

	// Vérifier que le contexte est annulé
	select {
	case <-monitor.ctx.Done():
		// Contexte annulé, c'est ce qu'on attend
	case <-time.After(100 * time.Millisecond):
		t.Error("Context should be cancelled after Stop()")
	}
}

// BenchmarkIsAzureAgentContainer benchmark de la détection
func BenchmarkIsAzureAgentContainer(b *testing.B) {
	monitor := &Monitor{}
	container := types.ContainerJSON{
		Config: &container.Config{
			Image: "myregistry/azure-agent:latest",
			Env: []string{
				"AZP_URL=https://dev.azure.com",
				"PATH=/usr/local/bin",
			},
			Labels: map[string]string{
				"app": "agent",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.IsAzureAgentContainer(container)
	}
}
