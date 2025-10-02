package ecsazrlc

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ECSNotifier gère la communication avec ECS pour signaler l'activité
type ECSNotifier struct {
	ecsClient            *ecs.Client
	ec2MetadataClient    *imds.Client
	clusterName          string
	taskARN              string
	containerInstanceARN string
	heartbeatInterval    time.Duration
	stopChan             chan struct{}
	ctx                  context.Context
}

// NewECSNotifier crée une nouvelle instance du notificateur ECS
func NewECSNotifier(clusterName string, heartbeatInterval time.Duration) (*ECSNotifier, error) {
	ctx := context.Background()

	// Charger la configuration AWS
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(getAWSRegion()))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	ecsClient := ecs.NewFromConfig(cfg)
	ec2MetadataClient := imds.NewFromConfig(cfg)

	notifier := &ECSNotifier{
		ecsClient:         ecsClient,
		ec2MetadataClient: ec2MetadataClient,
		clusterName:       clusterName,
		heartbeatInterval: heartbeatInterval,
		stopChan:          make(chan struct{}),
		ctx:               ctx,
	}

	// Récupérer les informations de l'instance
	if err := notifier.fetchInstanceInfo(); err != nil {
		log.Printf("Warning: Failed to fetch ECS instance info: %v", err)
	}

	return notifier, nil
}

// getAWSRegion retourne la région AWS depuis les variables d'environnement ou métadonnées
func getAWSRegion() string {
	if region := os.Getenv("AWS_REGION"); region != "" {
		return region
	}
	if region := os.Getenv("AWS_DEFAULT_REGION"); region != "" {
		return region
	}
	return "us-east-1" // Région par défaut
}

// fetchInstanceInfo récupère les informations de l'instance ECS
func (n *ECSNotifier) fetchInstanceInfo() error {
	// Récupérer l'instance ID depuis les métadonnées
	instanceIDOutput, err := n.ec2MetadataClient.GetMetadata(n.ctx, &imds.GetMetadataInput{
		Path: "instance-id",
	})
	if err != nil {
		return fmt.Errorf("failed to get instance ID: %w", err)
	}

	instanceID := instanceIDOutput.Content
	defer instanceID.Close()

	// Lire l'instance ID
	buf := make([]byte, 256)
	bytesRead, err := instanceID.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read instance ID: %w", err)
	}
	instanceIDStr := string(buf[:bytesRead])

	// Lister les instances du cluster pour trouver la nôtre
	input := &ecs.ListContainerInstancesInput{
		Cluster: aws.String(n.clusterName),
	}

	result, err := n.ecsClient.ListContainerInstances(n.ctx, input)
	if err != nil {
		return fmt.Errorf("failed to list container instances: %w", err)
	}

	if len(result.ContainerInstanceArns) == 0 {
		return fmt.Errorf("no container instances found in cluster")
	}

	// Décrire les instances pour trouver celle correspondant à notre EC2
	describeInput := &ecs.DescribeContainerInstancesInput{
		Cluster:            aws.String(n.clusterName),
		ContainerInstances: result.ContainerInstanceArns,
	}

	describeResult, err := n.ecsClient.DescribeContainerInstances(n.ctx, describeInput)
	if err != nil {
		return fmt.Errorf("failed to describe container instances: %w", err)
	}

	for _, instance := range describeResult.ContainerInstances {
		if instance.Ec2InstanceId != nil && *instance.Ec2InstanceId == instanceIDStr {
			n.containerInstanceARN = *instance.ContainerInstanceArn
			log.Printf("Found ECS container instance: %s", n.containerInstanceARN)
			return nil
		}
	}

	return fmt.Errorf("could not find container instance for EC2 instance %s", instanceIDStr)
}

// SendActivitySignal envoie un signal d'activité à ECS
func (n *ECSNotifier) SendActivitySignal(hasActivity bool) error {
	if n.containerInstanceARN == "" {
		log.Println("Container instance ARN not set, skipping ECS notification")
		return nil
	}

	// Mettre à jour les attributs de l'instance pour signaler l'activité
	timestamp := time.Now().Unix()
	activityStatus := "inactive"
	if hasActivity {
		activityStatus = "active"
	}

	input := &ecs.PutAttributesInput{
		Cluster: aws.String(n.clusterName),
		Attributes: []types.Attribute{
			{
				Name:  aws.String("azure-agent-activity"),
				Value: aws.String(activityStatus),
			},
			{
				Name:  aws.String("azure-agent-last-check"),
				Value: aws.String(fmt.Sprintf("%d", timestamp)),
			},
		},
	}

	_, err := n.ecsClient.PutAttributes(n.ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put attributes: %w", err)
	}

	log.Printf("Activity signal sent to ECS: %s (timestamp: %d)", activityStatus, timestamp)
	return nil
}

// StartHeartbeat démarre l'envoi périodique de signaux de vie
func (n *ECSNotifier) StartHeartbeat(monitor *Monitor) {
	ticker := time.NewTicker(n.heartbeatInterval)
	defer ticker.Stop()

	log.Printf("Starting ECS heartbeat every %v", n.heartbeatInterval)

	for {
		select {
		case <-ticker.C:
			hasActivity, err := monitor.HasActiveAgents()
			if err != nil {
				log.Printf("Error checking for active agents: %v", err)
				continue
			}

			if err := n.SendActivitySignal(hasActivity); err != nil {
				log.Printf("Error sending activity signal: %v", err)
			}

		case <-n.stopChan:
			log.Println("Heartbeat stopped")
			return
		}
	}
}

// NotifyActivity envoie immédiatement une notification d'activité
func (n *ECSNotifier) NotifyActivity(event ActivityEvent) error {
	log.Printf("Notifying ECS of Azure Agent activity: %s - %s", event.Action, event.ContainerName)
	return n.SendActivitySignal(true)
}

// SetProtectionEnabled active/désactive la protection contre la terminaison
func (n *ECSNotifier) SetProtectionEnabled(enabled bool) error {
	if n.containerInstanceARN == "" {
		return fmt.Errorf("container instance ARN not set")
	}

	var status types.ContainerInstanceStatus
	if enabled {
		status = types.ContainerInstanceStatusActive
	} else {
		status = types.ContainerInstanceStatusDraining
	}

	input := &ecs.UpdateContainerInstancesStateInput{
		Cluster:            aws.String(n.clusterName),
		ContainerInstances: []string{n.containerInstanceARN},
		Status:             status,
	}

	_, err := n.ecsClient.UpdateContainerInstancesState(n.ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update instance state: %w", err)
	}

	log.Printf("Instance protection set to: %v", enabled)
	return nil
}

// Stop arrête le notificateur
func (n *ECSNotifier) Stop() {
	close(n.stopChan)
}

// GetClusterInfo retourne des informations sur le cluster
func (n *ECSNotifier) GetClusterInfo() (map[string]interface{}, error) {
	input := &ecs.DescribeClustersInput{
		Clusters: []string{n.clusterName},
	}

	result, err := n.ecsClient.DescribeClusters(n.ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to describe cluster: %w", err)
	}

	if len(result.Clusters) == 0 {
		return nil, fmt.Errorf("cluster not found")
	}

	cluster := result.Clusters[0]
	info := map[string]interface{}{
		"name":                cluster.ClusterName,
		"status":              cluster.Status,
		"runningTasksCount":   cluster.RunningTasksCount,
		"pendingTasksCount":   cluster.PendingTasksCount,
		"activeServicesCount": cluster.ActiveServicesCount,
	}

	return info, nil
}
