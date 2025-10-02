# Documentation des Tests - ECS Azure Lifecircle

## Structure des Tests

Le module `ecsazrlc` contient trois types de tests:

### 1. Tests Unitaires

#### [monitor_test.go](monitor_test.go)
Tests pour la surveillance Docker des agents Azure DevOps:

- **TestNewMonitor**: Vérifie la création d'un moniteur
- **TestIsAzureAgentContainer**: Test de détection des conteneurs Azure Agent
  - Par nom d'image (azure-agent, azp, vsts)
  - Par variables d'environnement (AZP_*, VSTS_*)
  - Par labels Docker
  - Cas négatifs (non-agents)
- **TestActivityEvent**: Validation de la structure des événements
- **TestGetActivityChannel**: Test du canal de communication
- **TestHasActiveAgents**: Vérification de la détection d'agents actifs
- **TestMonitorStop**: Test d'arrêt propre du moniteur
- **BenchmarkIsAzureAgentContainer**: Benchmark de performance

#### [ecs_notifier_test.go](ecs_notifier_test.go)
Tests pour l'intégration AWS ECS:

- **TestGetAWSRegion**: Test de détection de la région AWS
  - Via AWS_REGION
  - Via AWS_DEFAULT_REGION
  - Région par défaut
- **TestECSNotifierCreation**: Test de création du notificateur
- **TestECSNotifierStop**: Test d'arrêt propre
- **TestNotifyActivity**: Test de notification d'activité
- **TestSendActivitySignalWithoutARN**: Comportement sans ARN d'instance
- **TestSetProtectionEnabledWithoutARN**: Gestion d'erreur sans ARN
- **TestHeartbeatStopSignal**: Test du cycle de vie du heartbeat
- **TestGetClusterInfoWithoutAWS**: Test sans connexion AWS
- **BenchmarkNotifyActivity**: Benchmark de notification

### 2. Tests d'Intégration

#### [integration_test.go](integration_test.go)
Tests nécessitant Docker et/ou AWS:

**Tests Docker:**
- **TestIntegrationMonitorWithDocker**: Test complet avec Docker
- **TestIntegrationMonitorActivityChannel**: Test du canal d'événements en conditions réelles
- **TestIntegrationConcurrentMonitoring**: Test de monitoring concurrent

**Tests AWS ECS:**
- **TestIntegrationECSNotifierWithAWS**: Test avec AWS ECS réel
- **TestIntegrationFullWorkflow**: Workflow complet Monitor + ECS
- **TestIntegrationHeartbeatLifecycle**: Test du cycle de vie du heartbeat

## Exécution des Tests

### Tests Unitaires Seulement
```bash
cd ecsazrlc
go test -v -short
```

### Tous les Tests (y compris intégration)
```bash
cd ecsazrlc
go test -v -tags=integration
```

### Tests avec Couverture
```bash
cd ecsazrlc
go test -v -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Tests Spécifiques
```bash
# Test uniquement monitor
go test -v -run TestMonitor

# Test uniquement ECS notifier
go test -v -run TestECS

# Tests d'intégration Docker
go test -v -tags=integration -run TestIntegrationMonitor
```

### Benchmarks
```bash
go test -bench=. -benchmem
```

## Configuration des Tests d'Intégration

### Prérequis Docker
Les tests d'intégration Docker nécessitent:
- Docker Engine installé et démarré
- Socket Docker accessible (`/var/run/docker.sock` sur Linux, named pipe sur Windows)
- Permissions pour accéder à Docker

### Prérequis AWS
Les tests d'intégration AWS nécessitent:

1. **Credentials AWS configurées**:
   ```bash
   export AWS_ACCESS_KEY_ID=your_key_id
   export AWS_SECRET_ACCESS_KEY=your_secret_key
   export AWS_REGION=us-east-1
   ```

2. **Cluster ECS configuré**:
   ```bash
   export ECS_CLUSTER_NAME=your-cluster-name
   ```

3. **Permissions IAM requises**:
   - `ecs:ListContainerInstances`
   - `ecs:DescribeContainerInstances`
   - `ecs:DescribeClusters`
   - `ecs:PutAttributes`
   - `ecs:UpdateContainerInstancesState`
   - `ec2:DescribeInstances` (pour métadonnées)

## Comportement des Tests

### Skip Automatique
Les tests skipent automatiquement dans ces cas:
- **Docker non disponible**: Tests monitor skip avec message explicatif
- **AWS credentials manquantes**: Tests ECS skip
- **Mode short** (`-short`): Tests d'intégration skippés
- **Variables d'environnement manquantes**: Tests AWS skippés

### Tests Mock
Certains tests utilisent des structures mock pour éviter les dépendances externes:
- Tests sans ARN ECS (simulent l'absence de configuration)
- Tests de cycle de vie sans appels réseau

## Structure de Couverture Attendue

| Composant | Couverture Cible | Notes |
|-----------|------------------|-------|
| monitor.go | >80% | Logique de détection critique |
| ecs_notifier.go | >70% | Certaines fonctions nécessitent AWS |
| cmd/main.go | >50% | Difficile à tester (CLI) |

## CI/CD Integration

### GitHub Actions Example
```yaml
test:
  runs-on: ubuntu-latest
  services:
    docker:
      image: docker:dind
  steps:
    - uses: actions/checkout@v3
    - uses: actions/setup-go@v4
      with:
        go-version: '1.24'

    # Tests unitaires
    - name: Unit Tests
      run: cd ecsazrlc && go test -v -short -cover

    # Tests d'intégration Docker (sans AWS)
    - name: Docker Integration Tests
      run: cd ecsazrlc && go test -v -tags=integration -run TestIntegrationMonitor

    # Tests AWS (avec secrets)
    - name: AWS Integration Tests
      if: github.event_name == 'push' && github.ref == 'refs/heads/main'
      env:
        AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
        AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
        AWS_REGION: us-east-1
        ECS_CLUSTER_NAME: test-cluster
      run: cd ecsazrlc && go test -v -tags=integration -run TestIntegrationECS
```

## Debugging des Tests

### Logs Verbeux
```bash
go test -v -run TestName
```

### Tests avec Race Detector
```bash
go test -race -v
```

### Profiling Mémoire
```bash
go test -memprofile=mem.out
go tool pprof mem.out
```

### Profiling CPU
```bash
go test -cpuprofile=cpu.out
go tool pprof cpu.out
```

## Bonnes Pratiques

1. **Tests isolés**: Chaque test doit être indépendant
2. **Cleanup**: Utiliser `defer` pour le nettoyage
3. **Skip intelligemment**: Utiliser `t.Skip()` au lieu de `t.Fatal()` pour les dépendances manquantes
4. **Logs informatifs**: Logger les informations utiles avec `t.Logf()`
5. **Timeouts**: Utiliser des timeouts pour éviter les tests bloqués
6. **Contexts**: Utiliser `context.WithTimeout` pour les opérations longues

## Troubleshooting

### "Docker not available"
- Vérifier que Docker est démarré: `docker ps`
- Vérifier les permissions: `sudo usermod -aG docker $USER`
- Redémarrer la session

### "AWS credentials not available"
- Vérifier les credentials: `aws configure list`
- Tester l'accès: `aws ecs describe-clusters --cluster your-cluster`
- Vérifier les permissions IAM

### Tests lents
- Utiliser `-short` pour les tests rapides
- Réduire les timeouts dans les tests d'intégration
- Paralléliser avec `-parallel N`

### Tests flaky
- Augmenter les timeouts
- Vérifier les race conditions avec `-race`
- Ajouter des retries pour les opérations réseau
