package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hypolas/ecsazrlc"
)

func main() {
	// Flags de ligne de commande
	clusterName := flag.String("cluster", "", "Nom du cluster ECS (requis si mode ECS activé)")
	heartbeatInterval := flag.Duration("heartbeat", 30*time.Second, "Intervalle entre les heartbeats ECS")
	enableECS := flag.Bool("enable-ecs", false, "Activer les notifications ECS")
	monitorOnly := flag.Bool("monitor-only", false, "Mode monitoring seul sans ECS")
	verbose := flag.Bool("verbose", false, "Mode verbose")
	flag.Parse()

	if *verbose {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
	}

	log.Println("=== ECS Azure Lifecircle Monitor ===")
	log.Printf("Version: 1.0.0")
	log.Printf("Monitor mode: %v", *monitorOnly)
	log.Printf("ECS integration: %v", *enableECS)

	// Validation
	if *enableECS && !*monitorOnly && *clusterName == "" {
		log.Fatal("Le nom du cluster ECS est requis avec --enable-ecs (utilisez --cluster)")
	}

	// Créer le moniteur Docker
	monitor, err := ecsazrlc.NewMonitor()
	if err != nil {
		log.Fatalf("Failed to create monitor: %v", err)
	}
	defer monitor.Stop()

	log.Println("Docker monitor initialized successfully")

	// Démarrer le monitoring
	if err := monitor.StartMonitoring(); err != nil {
		log.Fatalf("Failed to start monitoring: %v", err)
	}

	log.Println("Docker event monitoring started")

	// Créer le notificateur ECS si activé
	var notifier *ecsazrlc.ECSNotifier
	if *enableECS && !*monitorOnly {
		notifier, err = ecsazrlc.NewECSNotifier(*clusterName, *heartbeatInterval)
		if err != nil {
			log.Printf("Warning: Failed to create ECS notifier: %v", err)
			log.Println("Continuing in monitor-only mode...")
		} else {
			log.Printf("ECS notifier initialized for cluster: %s", *clusterName)

			// Afficher les informations du cluster
			clusterInfo, err := notifier.GetClusterInfo()
			if err != nil {
				log.Printf("Warning: Could not fetch cluster info: %v", err)
			} else {
				log.Printf("Cluster info: %+v", clusterInfo)
			}

			// Démarrer le heartbeat
			go notifier.StartHeartbeat(monitor)
			log.Printf("Heartbeat started with interval: %v", *heartbeatInterval)
		}
	}

	// Écouter les événements d'activité
	go func() {
		activityChan := monitor.GetActivityChannel()
		for event := range activityChan {
			log.Printf("[ACTIVITY] Container: %s (%s) - Action: %s - Image: %s",
				event.ContainerName,
				event.ContainerID,
				event.Action,
				event.ImageName)

			// Notifier ECS en cas d'activité importante
			if notifier != nil && (event.Action == "start" || event.Action == "exec_start") {
				if err := notifier.NotifyActivity(event); err != nil {
					log.Printf("Error notifying ECS: %v", err)
				}
			}
		}
	}()

	log.Println("=== Monitoring active - Press Ctrl+C to stop ===")

	// Attendre le signal d'arrêt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("\nShutdown signal received, stopping...")

	// Arrêter proprement
	if notifier != nil {
		notifier.Stop()
	}
	monitor.Stop()

	log.Println("Application stopped successfully")
}
