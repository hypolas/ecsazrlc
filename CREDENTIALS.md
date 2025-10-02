# Configuration des Credentials AWS

Le module utilise **aws-sdk-go-v2** qui gère automatiquement les credentials via une chaîne de providers.

## Ordre de recherche des credentials

Le SDK cherche les credentials dans cet ordre :

### 1. Variables d'environnement (recommandé pour dev/test)

```bash
export AWS_ACCESS_KEY_ID="votre-access-key"
export AWS_SECRET_ACCESS_KEY="votre-secret-key"
export AWS_SESSION_TOKEN="votre-token"  # Optionnel, pour credentials temporaires
export AWS_REGION="eu-west-1"
```

### 2. Fichier de credentials AWS (~/.aws/credentials)

```ini
[default]
aws_access_key_id = votre-access-key
aws_secret_access_key = votre-secret-key

[production]
aws_access_key_id = autre-access-key
aws_secret_access_key = autre-secret-key
```

Utilisation d'un profil spécifique :
```bash
export AWS_PROFILE=production
./ecsazrlc --enable-ecs --cluster mon-cluster
```

### 3. IAM Role (RECOMMANDÉ pour production sur EC2/ECS)

**C'est la méthode la plus sécurisée et automatique !**

Aucune credential à fournir, le SDK récupère automatiquement les credentials depuis :
- **EC2 Instance Metadata Service** (IMDS)
- **ECS Task Role**

#### Configuration du rôle IAM pour EC2/ECS

Créez un rôle IAM avec cette policy :

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ECSMonitoringPermissions",
      "Effect": "Allow",
      "Action": [
        "ecs:DescribeClusters",
        "ecs:ListContainerInstances",
        "ecs:DescribeContainerInstances",
        "ecs:PutAttributes",
        "ecs:UpdateContainerInstancesState"
      ],
      "Resource": "*"
    },
    {
      "Sid": "EC2MetadataAccess",
      "Effect": "Allow",
      "Action": [
        "ec2:DescribeInstances"
      ],
      "Resource": "*"
    }
  ]
}
```

Attachez ce rôle à :
- **Instance EC2** : Lors de la création ou via "Actions > Security > Modify IAM role"
- **ECS Task** : Dans la task definition, champ `taskRoleArn`

### 4. ECS Container Credentials

Si l'application tourne dans un conteneur ECS, le SDK utilise automatiquement :
```
AWS_CONTAINER_CREDENTIALS_RELATIVE_URI
```

## Scénarios d'utilisation

### Développement local

```bash
# Avec credentials explicites
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
export AWS_REGION="eu-west-1"

./ecsazrlc --monitor-only
```

### Test avec profil AWS

```bash
# Utiliser le profil "dev" de ~/.aws/credentials
AWS_PROFILE=dev ./ecsazrlc --enable-ecs --cluster dev-cluster
```

### Production sur EC2 (AUCUNE credential nécessaire)

```bash
# L'instance EC2 a un rôle IAM attaché
# Le SDK récupère automatiquement les credentials via IMDS
./ecsazrlc --enable-ecs --cluster prod-cluster --heartbeat 30s
```

### Dans un conteneur Docker sur EC2

```bash
# Le socket Docker doit être monté
docker run -d \
  --name ecsazrlc \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e AWS_REGION=eu-west-1 \
  ecsazrlc:latest \
  --enable-ecs --cluster prod-cluster
```

**Note** : Le conteneur héritera des credentials de l'instance EC2 via IMDS.

### Dans ECS Task

Dans votre task definition :

```json
{
  "family": "ecsazrlc-task",
  "taskRoleArn": "arn:aws:iam::123456789012:role/ecsazrlc-task-role",
  "executionRoleArn": "arn:aws:iam::123456789012:role/ecsTaskExecutionRole",
  "containerDefinitions": [
    {
      "name": "ecsazrlc",
      "image": "ecsazrlc:latest",
      "essential": true,
      "environment": [
        {
          "name": "AWS_REGION",
          "value": "eu-west-1"
        }
      ],
      "mountPoints": [
        {
          "sourceVolume": "docker-socket",
          "containerPath": "/var/run/docker.sock"
        }
      ],
      "command": [
        "--enable-ecs",
        "--cluster", "prod-cluster",
        "--heartbeat", "30s"
      ]
    }
  ],
  "volumes": [
    {
      "name": "docker-socket",
      "host": {
        "sourcePath": "/var/run/docker.sock"
      }
    }
  ]
}
```

## Vérification des credentials

Pour tester si les credentials fonctionnent :

```bash
# Avec AWS CLI
aws sts get-caller-identity

# Devrait retourner :
# {
#   "UserId": "AIDAI...",
#   "Account": "123456789012",
#   "Arn": "arn:aws:iam::123456789012:user/votre-user"
# }
```

## Sécurité

### ✅ Bonnes pratiques

- **Production** : Toujours utiliser IAM Roles (pas de credentials en dur)
- **Développement** : Variables d'environnement ou fichier `~/.aws/credentials`
- **Jamais** : Hardcoder les credentials dans le code
- **Jamais** : Commiter les credentials dans Git

### ⚠️ À éviter

```go
// ❌ NE JAMAIS FAIRE ÇA
cfg, err := config.LoadDefaultConfig(ctx,
    config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
        "AKIAIOSFODNN7EXAMPLE",
        "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
        "",
    )),
)
```

## Dépannage

### Erreur : "NoCredentialProviders: no valid providers in chain"

**Cause** : Aucune credential trouvée.

**Solution** :
1. Vérifier les variables d'environnement : `env | grep AWS`
2. Vérifier le fichier : `cat ~/.aws/credentials`
3. Sur EC2 : Vérifier le rôle IAM attaché
4. Tester : `aws sts get-caller-identity`

### Erreur : "AccessDenied"

**Cause** : Les credentials existent mais n'ont pas les bonnes permissions.

**Solution** : Vérifier/mettre à jour la policy IAM avec les permissions requises.

### Sur Windows

Le fichier credentials se trouve à :
```
C:\Users\VotreUtilisateur\.aws\credentials
```

## Résumé

| Environnement | Méthode recommandée | Configuration requise |
|---------------|---------------------|----------------------|
| Développement local | Variables d'env ou fichier credentials | Export AWS_* ou ~/.aws/credentials |
| EC2 Production | IAM Instance Role | Rôle IAM attaché à l'instance |
| ECS Production | IAM Task Role | taskRoleArn dans task definition |
| CI/CD | Variables d'env | Secrets dans le pipeline |

**Dans tous les cas, le code n'a pas besoin d'être modifié !** Le SDK gère automatiquement la récupération des credentials.
