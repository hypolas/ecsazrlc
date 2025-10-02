.PHONY: help build run stop clean test logs

help: ## Afficher l'aide
	@echo "Commandes disponibles:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build l'image Docker
	docker-compose build

run: ## Démarrer le service en mode monitoring
	docker-compose up -d

run-ecs: ## Démarrer avec intégration ECS (nécessite .env avec ECS_CLUSTER_NAME)
	@if [ -z "$(ECS_CLUSTER_NAME)" ]; then \
		echo "Error: ECS_CLUSTER_NAME non défini. Utilisez: make run-ecs ECS_CLUSTER_NAME=votre-cluster"; \
		exit 1; \
	fi
	docker-compose run -d ecsazrlc --enable-ecs --cluster $(ECS_CLUSTER_NAME) --heartbeat 30s

run-dev: ## Démarrer en mode verbose pour développement
	docker-compose run --rm ecsazrlc --monitor-only --verbose

stop: ## Arrêter les services
	docker-compose down

restart: stop run ## Redémarrer les services

logs: ## Afficher les logs
	docker-compose logs -f ecsazrlc

logs-all: ## Afficher tous les logs
	docker-compose logs -f

clean: ## Nettoyer les conteneurs et images
	docker-compose down -v
	docker rmi ecsazrlc:latest 2>/dev/null || true

test: ## Build et test rapide
	cd ecsazrlc && go test ./...

build-local: ## Build le binaire localement
	cd ecsazrlc && go build -o ecsazrlc ./cmd

run-local: build-local ## Exécuter localement (sans Docker)
	cd ecsazrlc && ./ecsazrlc --monitor-only --verbose

# Commandes Windows
build-local-windows: ## Build le binaire pour Windows
	cd ecsazrlc && set GOOS=windows&& set GOARCH=amd64&& go build -o ecsazrlc.exe ./cmd

# Docker compose avec agent Azure de test
test-with-agent: ## Démarrer avec un agent Azure de test
	docker-compose --profile test up -d

ps: ## Afficher les conteneurs en cours
	docker-compose ps

stats: ## Afficher les stats des conteneurs
	docker stats ecsazrlc
