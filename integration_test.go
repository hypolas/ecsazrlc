// +build integration

package ecsazrlc

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestIntegrationMonitorWithDocker teste l'intégration avec Docker
func TestIntegrationMonitorWithDocker(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	monitor, err := NewMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.Stop()

	// Démarrer le monitoring
	err = monitor.StartMonitoring()
	if err != nil {
		t.Fatalf("Failed to start monitoring: %v", err)
	}

	// Vérifier la détection d'agents
	agents, err := monitor.GetRunningAzureAgents()
	if err != nil {
		t.Fatalf("Failed to get running agents: %v", err)
	}

	t.Logf("Found %d running Azure agents", len(agents))

	// Vérifier HasActiveAgents
	hasActive, err := monitor.HasActiveAgents()
	if err != nil {
		t.Fatalf("Failed to check active agents: %v", err)
	}

	if hasActive && len(agents) == 0 {
		t.Error("HasActiveAgents returned true but GetRunningAzureAgents returned empty list")
	}

	if !hasActive && len(agents) > 0 {
		t.Error("HasActiveAgents returned false but GetRunningAzureAgents found agents")
	}
}

// TestIntegrationMonitorActivityChannel teste le canal d'événements
func TestIntegrationMonitorActivityChannel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	monitor, err := NewMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.Stop()

	err = monitor.StartMonitoring()
	if err != nil {
		t.Fatalf("Failed to start monitoring: %v", err)
	}

	// Écouter les événements pendant 5 secondes
	timeout := time.After(5 * time.Second)
	eventCount := 0

	activityChan := monitor.GetActivityChannel()

	for {
		select {
		case event := <-activityChan:
			t.Logf("Received activity event: %s - %s [%s]",
				event.Action, event.ContainerName, event.ContainerID)
			eventCount++

			// Vérifier la structure de l'événement
			if event.ContainerID == "" {
				t.Error("Event should have a ContainerID")
			}
			if event.Timestamp.IsZero() {
				t.Error("Event should have a Timestamp")
			}

		case <-timeout:
			t.Logf("Monitoring completed. Received %d events", eventCount)
			return
		}
	}
}

// TestIntegrationECSNotifierWithAWS teste l'intégration avec AWS ECS
func TestIntegrationECSNotifierWithAWS(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Vérifier les variables d'environnement
	clusterName := os.Getenv("ECS_CLUSTER_NAME")
	if clusterName == "" {
		t.Skip("ECS_CLUSTER_NAME not set, skipping AWS integration test")
	}

	// Créer le notificateur
	notifier, err := NewECSNotifier(clusterName, 30*time.Second)
	if err != nil {
		t.Fatalf("Failed to create ECS notifier: %v", err)
	}
	defer notifier.Stop()

	// Récupérer les informations du cluster
	clusterInfo, err := notifier.GetClusterInfo()
	if err != nil {
		t.Fatalf("Failed to get cluster info: %v", err)
	}

	t.Logf("Cluster info: %+v", clusterInfo)

	// Vérifier la structure des informations
	if name, ok := clusterInfo["name"]; !ok || name == nil {
		t.Error("Cluster info should contain 'name'")
	}

	if status, ok := clusterInfo["status"]; !ok || status == nil {
		t.Error("Cluster info should contain 'status'")
	}
}

// TestIntegrationFullWorkflow teste le workflow complet
func TestIntegrationFullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Créer le monitor
	monitor, err := NewMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.Stop()

	// Démarrer le monitoring
	err = monitor.StartMonitoring()
	if err != nil {
		t.Fatalf("Failed to start monitoring: %v", err)
	}

	// Vérifier la présence d'agents
	hasAgents, err := monitor.HasActiveAgents()
	if err != nil {
		t.Fatalf("Failed to check active agents: %v", err)
	}

	t.Logf("Has active Azure agents: %v", hasAgents)

	// Si on a des credentials AWS et un cluster configuré
	clusterName := os.Getenv("ECS_CLUSTER_NAME")
	if clusterName != "" {
		notifier, err := NewECSNotifier(clusterName, 10*time.Second)
		if err != nil {
			t.Logf("Warning: Failed to create ECS notifier: %v", err)
		} else {
			defer notifier.Stop()

			// Tester l'envoi d'un signal d'activité
			err = notifier.SendActivitySignal(hasAgents)
			if err != nil {
				t.Logf("Warning: Failed to send activity signal: %v", err)
			}

			// Tester une notification d'événement
			if hasAgents {
				agents, _ := monitor.GetRunningAzureAgents()
				if len(agents) > 0 {
					err = notifier.NotifyActivity(agents[0])
					if err != nil {
						t.Logf("Warning: Failed to notify activity: %v", err)
					}
				}
			}
		}
	}

	// Attendre quelques événements
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	eventCount := 0
	activityChan := monitor.GetActivityChannel()

	for {
		select {
		case event := <-activityChan:
			t.Logf("Received event: %s - %s", event.Action, event.ContainerName)
			eventCount++
		case <-ctx.Done():
			t.Logf("Workflow completed. Processed %d events", eventCount)
			return
		}
	}
}

// TestIntegrationConcurrentMonitoring teste le monitoring concurrent
func TestIntegrationConcurrentMonitoring(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	const numMonitors = 3

	monitors := make([]*Monitor, numMonitors)
	for i := 0; i < numMonitors; i++ {
		monitor, err := NewMonitor()
		if err != nil {
			t.Fatalf("Failed to create monitor %d: %v", i, err)
		}
		defer monitor.Stop()

		err = monitor.StartMonitoring()
		if err != nil {
			t.Fatalf("Failed to start monitoring %d: %v", i, err)
		}

		monitors[i] = monitor
	}

	// Attendre un peu
	time.Sleep(2 * time.Second)

	// Vérifier que tous les monitors fonctionnent
	for i, monitor := range monitors {
		hasAgents, err := monitor.HasActiveAgents()
		if err != nil {
			t.Errorf("Monitor %d failed to check agents: %v", i, err)
		}
		t.Logf("Monitor %d: Has active agents = %v", i, hasAgents)
	}
}

// TestIntegrationHeartbeatLifecycle teste le cycle de vie du heartbeat
func TestIntegrationHeartbeatLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	clusterName := os.Getenv("ECS_CLUSTER_NAME")
	if clusterName == "" {
		t.Skip("ECS_CLUSTER_NAME not set")
	}

	monitor, err := NewMonitor()
	if err != nil {
		t.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.Stop()

	err = monitor.StartMonitoring()
	if err != nil {
		t.Fatalf("Failed to start monitoring: %v", err)
	}

	notifier, err := NewECSNotifier(clusterName, 2*time.Second)
	if err != nil {
		t.Skipf("Failed to create notifier: %v", err)
	}
	defer notifier.Stop()

	// Démarrer le heartbeat
	done := make(chan bool)
	go func() {
		notifier.StartHeartbeat(monitor)
		done <- true
	}()

	// Attendre 3 cycles de heartbeat
	time.Sleep(7 * time.Second)

	// Arrêter le heartbeat
	notifier.Stop()

	// Vérifier l'arrêt propre
	select {
	case <-done:
		t.Log("Heartbeat stopped cleanly")
	case <-time.After(2 * time.Second):
		t.Error("Heartbeat did not stop in time")
	}
}
