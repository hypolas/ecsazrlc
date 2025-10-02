package ecsazrlc

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

// Monitor surveille l'activité des conteneurs Azure DevOps Agent
type Monitor struct {
	dockerClient      *client.Client
	ctx               context.Context
	cancel            context.CancelFunc
	activityChan      chan ActivityEvent
	excludeContainers []string // Liste des noms/IDs de conteneurs à exclure
	excludeImages     []string // Liste des images à exclure
}

// ActivityEvent représente un événement d'activité
type ActivityEvent struct {
	ContainerID   string
	ContainerName string
	ImageName     string
	Action        string
	Timestamp     time.Time
	IsAzureAgent  bool
}

// MonitorConfig contient la configuration du moniteur
type MonitorConfig struct {
	ExcludeContainers []string // Noms ou IDs de conteneurs à exclure
	ExcludeImages     []string // Images à exclure (patterns)
}

// NewMonitor crée une nouvelle instance du moniteur
func NewMonitor() (*Monitor, error) {
	return NewMonitorWithConfig(MonitorConfig{})
}

// NewMonitorWithConfig crée une nouvelle instance du moniteur avec configuration
func NewMonitorWithConfig(config MonitorConfig) (*Monitor, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Monitor{
		dockerClient:      cli,
		ctx:               ctx,
		cancel:            cancel,
		activityChan:      make(chan ActivityEvent, 100),
		excludeContainers: config.ExcludeContainers,
		excludeImages:     config.ExcludeImages,
	}, nil
}

// isExcluded vérifie si un conteneur doit être exclu
func (m *Monitor) isExcluded(containerID, containerName, imageName string) bool {
	// Vérifier l'exclusion par nom ou ID de conteneur
	for _, excluded := range m.excludeContainers {
		if strings.Contains(containerName, excluded) || strings.Contains(containerID, excluded) {
			log.Printf("Container %s (%s) excluded by container filter: %s", containerName, containerID[:12], excluded)
			return true
		}
	}

	// Vérifier l'exclusion par image
	for _, excluded := range m.excludeImages {
		if strings.Contains(imageName, excluded) {
			log.Printf("Container %s (%s) excluded by image filter: %s", containerName, containerID[:12], excluded)
			return true
		}
	}

	return false
}

// IsAzureAgentContainer vérifie si un conteneur est un agent Azure DevOps
func (m *Monitor) IsAzureAgentContainer(containerInfo types.ContainerJSON) bool {
	// Vérifier l'image
	imageName := strings.ToLower(containerInfo.Config.Image)
	if strings.Contains(imageName, "azure") && strings.Contains(imageName, "agent") {
		return true
	}
	if strings.Contains(imageName, "azp") || strings.Contains(imageName, "vsts") {
		return true
	}

	// Vérifier les labels
	for key, value := range containerInfo.Config.Labels {
		lowerKey := strings.ToLower(key)
		lowerValue := strings.ToLower(value)
		if strings.Contains(lowerKey, "azure") || strings.Contains(lowerValue, "azure") {
			if strings.Contains(lowerKey, "agent") || strings.Contains(lowerValue, "agent") {
				return true
			}
		}
	}

	// Vérifier les variables d'environnement
	for _, env := range containerInfo.Config.Env {
		lowerEnv := strings.ToLower(env)
		if strings.Contains(lowerEnv, "azp_") || strings.Contains(lowerEnv, "vsts_") {
			return true
		}
	}

	return false
}

// GetRunningAzureAgents retourne la liste des agents Azure actuellement en cours d'exécution
func (m *Monitor) GetRunningAzureAgents() ([]ActivityEvent, error) {
	containers, err := m.dockerClient.ContainerList(m.ctx, container.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var agents []ActivityEvent
	for _, c := range containers {
		name := strings.TrimPrefix(c.Names[0], "/")

		// Vérifier si le conteneur est exclu
		if m.isExcluded(c.ID, name, c.Image) {
			continue
		}

		containerInfo, err := m.dockerClient.ContainerInspect(m.ctx, c.ID)
		if err != nil {
			log.Printf("Warning: failed to inspect container %s: %v", c.ID, err)
			continue
		}

		if m.IsAzureAgentContainer(containerInfo) {
			agents = append(agents, ActivityEvent{
				ContainerID:   c.ID[:12],
				ContainerName: name,
				ImageName:     c.Image,
				Action:        "running",
				Timestamp:     time.Now(),
				IsAzureAgent:  true,
			})
		}
	}

	return agents, nil
}

// StartMonitoring démarre la surveillance des événements Docker
func (m *Monitor) StartMonitoring() error {
	// Vérifier d'abord les conteneurs en cours d'exécution
	initialAgents, err := m.GetRunningAzureAgents()
	if err != nil {
		return fmt.Errorf("failed to get initial agents: %w", err)
	}

	log.Printf("Found %d Azure agent container(s) currently running", len(initialAgents))
	for _, agent := range initialAgents {
		m.activityChan <- agent
	}

	// Écouter les événements Docker
	eventsChan, errChan := m.dockerClient.Events(m.ctx, events.ListOptions{
		Since: fmt.Sprintf("%d", time.Now().Add(-1*time.Minute).Unix()), // Éviter de manquer les événements récents
	})

	go func() {
		for {
			select {
			case <-m.ctx.Done():
				log.Println("Monitoring stopped")
				return

			case err := <-errChan:
				if err != nil && err != io.EOF {
					log.Printf("Error receiving Docker events: %v", err)
				}
				return

			case event := <-eventsChan:
				m.handleDockerEvent(event)
			}
		}
	}()

	return nil
}

// handleDockerEvent traite un événement Docker
func (m *Monitor) handleDockerEvent(event events.Message) {
	if event.Type != events.ContainerEventType {
		return
	}

	// Événements intéressants pour détecter l'activité
	interestingActions := map[string]bool{
		"start":      true,
		"die":        true,
		"stop":       true,
		"kill":       true,
		"create":     true,
		"exec_start": true,
	}

	if !interestingActions[string(event.Action)] {
		return
	}

	// Inspecter le conteneur pour vérifier s'il s'agit d'un agent Azure
	containerInfo, err := m.dockerClient.ContainerInspect(m.ctx, event.Actor.ID)
	if err != nil {
		// Le conteneur peut avoir été supprimé
		if event.Action == "die" || event.Action == "stop" || event.Action == "kill" {
			log.Printf("Container %s already removed", event.Actor.ID[:12])
			return
		}
		log.Printf("Failed to inspect container %s: %v", event.Actor.ID[:12], err)
		return
	}

	name := event.Actor.Attributes["name"]
	image := event.Actor.Attributes["image"]

	// Vérifier si le conteneur est exclu
	if m.isExcluded(event.Actor.ID, name, image) {
		return
	}

	isAzureAgent := m.IsAzureAgentContainer(containerInfo)
	if !isAzureAgent {
		return
	}

	activityEvent := ActivityEvent{
		ContainerID:   event.Actor.ID[:12],
		ContainerName: name,
		ImageName:     image,
		Action:        string(event.Action),
		Timestamp:     time.Unix(event.Time, 0),
		IsAzureAgent:  true,
	}

	log.Printf("Azure Agent Activity: %s - %s [%s]", activityEvent.Action, activityEvent.ContainerName, activityEvent.ContainerID)
	m.activityChan <- activityEvent
}

// GetActivityChannel retourne le canal des événements d'activité
func (m *Monitor) GetActivityChannel() <-chan ActivityEvent {
	return m.activityChan
}

// HasActiveAgents vérifie s'il y a des agents Azure actifs
func (m *Monitor) HasActiveAgents() (bool, error) {
	agents, err := m.GetRunningAzureAgents()
	if err != nil {
		return false, err
	}
	return len(agents) > 0, nil
}

// Stop arrête le monitoring
func (m *Monitor) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.dockerClient != nil {
		m.dockerClient.Close()
	}
	close(m.activityChan)
}
