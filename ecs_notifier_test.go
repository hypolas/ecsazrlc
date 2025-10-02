package ecsazrlc

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestGetAWSRegion vérifie la récupération de la région AWS
func TestGetAWSRegion(t *testing.T) {
	tests := []struct {
		name           string
		awsRegion      string
		awsDefaultRegion string
		expected       string
	}{
		{
			name:      "AWS_REGION set",
			awsRegion: "eu-west-1",
			expected:  "eu-west-1",
		},
		{
			name:             "AWS_DEFAULT_REGION set",
			awsDefaultRegion: "us-west-2",
			expected:         "us-west-2",
		},
		{
			name:     "Default region",
			expected: "us-east-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sauvegarder les variables d'environnement
			oldRegion := os.Getenv("AWS_REGION")
			oldDefaultRegion := os.Getenv("AWS_DEFAULT_REGION")
			defer func() {
				os.Setenv("AWS_REGION", oldRegion)
				os.Setenv("AWS_DEFAULT_REGION", oldDefaultRegion)
			}()

			// Nettoyer les variables
			os.Unsetenv("AWS_REGION")
			os.Unsetenv("AWS_DEFAULT_REGION")

			// Définir les nouvelles valeurs
			if tt.awsRegion != "" {
				os.Setenv("AWS_REGION", tt.awsRegion)
			}
			if tt.awsDefaultRegion != "" {
				os.Setenv("AWS_DEFAULT_REGION", tt.awsDefaultRegion)
			}

			result := getAWSRegion()
			if result != tt.expected {
				t.Errorf("getAWSRegion() = %s, want %s", result, tt.expected)
			}
		})
	}
}

// TestECSNotifierCreation vérifie la création du notificateur
func TestECSNotifierCreation(t *testing.T) {
	// Ce test nécessite des credentials AWS valides
	// Il sera skip si elles ne sont pas disponibles

	// Vérifier si les credentials sont disponibles
	region := os.Getenv("AWS_REGION")
	if region == "" {
		os.Setenv("AWS_REGION", "us-east-1")
		defer os.Unsetenv("AWS_REGION")
	}

	notifier, err := NewECSNotifier("test-cluster", 30*time.Second)

	// Si on n'a pas de credentials valides, le test devrait skip
	if err != nil {
		t.Skipf("Skipping test: AWS credentials not available - %v", err)
	}

	if notifier == nil {
		t.Fatal("Expected notifier to be created, got nil")
	}

	if notifier.clusterName != "test-cluster" {
		t.Errorf("Expected cluster name 'test-cluster', got %s", notifier.clusterName)
	}

	if notifier.heartbeatInterval != 30*time.Second {
		t.Errorf("Expected interval 30s, got %v", notifier.heartbeatInterval)
	}

	defer notifier.Stop()
}

// TestECSNotifierStop vérifie l'arrêt du notificateur
func TestECSNotifierStop(t *testing.T) {
	notifier := &ECSNotifier{
		stopChan: make(chan struct{}),
		ctx:      context.Background(),
	}

	// Canal ouvert
	select {
	case <-notifier.stopChan:
		t.Error("stopChan should not be closed yet")
	default:
		// OK
	}

	notifier.Stop()

	// Canal fermé après Stop
	select {
	case <-notifier.stopChan:
		// OK - canal fermé
	case <-time.After(100 * time.Millisecond):
		t.Error("stopChan should be closed after Stop()")
	}
}

// TestNotifyActivity vérifie la notification d'activité
func TestNotifyActivity(t *testing.T) {
	// Mock notifier sans connexion AWS réelle
	notifier := &ECSNotifier{
		clusterName:          "test-cluster",
		containerInstanceARN: "", // Pas d'ARN, devrait skip l'envoi
		ctx:                  context.Background(),
		stopChan:             make(chan struct{}),
	}

	event := ActivityEvent{
		ContainerID:   "test123",
		ContainerName: "azure-agent-1",
		ImageName:     "azure-agent:latest",
		Action:        "start",
		Timestamp:     time.Now(),
		IsAzureAgent:  true,
	}

	// Sans ARN, devrait retourner nil sans erreur
	err := notifier.NotifyActivity(event)
	if err != nil {
		t.Errorf("NotifyActivity() should not error without ARN: %v", err)
	}
}

// TestSendActivitySignalWithoutARN vérifie le comportement sans ARN
func TestSendActivitySignalWithoutARN(t *testing.T) {
	notifier := &ECSNotifier{
		containerInstanceARN: "",
		ctx:                  context.Background(),
	}

	// Sans ARN, devrait retourner nil
	err := notifier.SendActivitySignal(true)
	if err != nil {
		t.Errorf("SendActivitySignal() should not error without ARN: %v", err)
	}
}

// TestSetProtectionEnabledWithoutARN vérifie l'erreur sans ARN
func TestSetProtectionEnabledWithoutARN(t *testing.T) {
	notifier := &ECSNotifier{
		containerInstanceARN: "",
		ctx:                  context.Background(),
	}

	// Sans ARN, devrait retourner une erreur
	err := notifier.SetProtectionEnabled(true)
	if err == nil {
		t.Error("SetProtectionEnabled() should error without ARN")
	}
}

// TestHeartbeatStopSignal vérifie l'arrêt du heartbeat
func TestHeartbeatStopSignal(t *testing.T) {
	notifier := &ECSNotifier{
		heartbeatInterval:    100 * time.Millisecond,
		stopChan:             make(chan struct{}),
		ctx:                  context.Background(),
		containerInstanceARN: "", // Pas d'ARN pour éviter les appels AWS
	}

	// Créer un vrai monitor pour éviter nil pointer
	monitor, err := NewMonitor()
	if err != nil {
		t.Skipf("Skipping test: Docker not available - %v", err)
	}
	defer monitor.Stop()

	// Démarrer le heartbeat dans une goroutine
	done := make(chan bool)
	go func() {
		notifier.StartHeartbeat(monitor)
		done <- true
	}()

	// Attendre un peu
	time.Sleep(150 * time.Millisecond)

	// Arrêter le heartbeat
	notifier.Stop()

	// Vérifier que le heartbeat s'est arrêté
	select {
	case <-done:
		// OK - heartbeat arrêté
	case <-time.After(500 * time.Millisecond):
		t.Error("Heartbeat should stop after Stop() is called")
	}
}

// TestGetClusterInfoWithoutAWS vérifie le comportement sans AWS
func TestGetClusterInfoWithoutAWS(t *testing.T) {
	// Tenter de créer un notificateur réel
	notifier, err := NewECSNotifier("test-cluster", 30*time.Second)
	if err != nil {
		t.Skipf("Skipping test: AWS not available - %v", err)
	}
	defer notifier.Stop()

	// Tenter de récupérer les infos du cluster
	info, err := notifier.GetClusterInfo()

	// Sans cluster réel, devrait échouer
	if err != nil {
		t.Logf("Expected error without real cluster: %v", err)
		return
	}

	// Si on a un résultat, vérifier la structure
	if info != nil {
		if _, ok := info["name"]; !ok {
			t.Error("Expected 'name' field in cluster info")
		}
		if _, ok := info["status"]; !ok {
			t.Error("Expected 'status' field in cluster info")
		}
	}
}

// BenchmarkNotifyActivity benchmark de la notification
func BenchmarkNotifyActivity(b *testing.B) {
	notifier := &ECSNotifier{
		containerInstanceARN: "", // Pas d'ARN = pas d'appel AWS
		ctx:                  context.Background(),
	}

	event := ActivityEvent{
		ContainerID:   "test123",
		ContainerName: "azure-agent-1",
		Action:        "start",
		Timestamp:     time.Now(),
		IsAzureAgent:  true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		notifier.NotifyActivity(event)
	}
}
