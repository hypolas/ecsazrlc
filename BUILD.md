# Build and Deployment Guide

## Prerequisites

- Docker and Docker Compose installed
- Go 1.25.1+ (for local build)
- Access to Docker socket (`/var/run/docker.sock`)

## Quick Start with Docker Compose

### 1. Build the image

```bash
# With Docker Compose
docker-compose build

# Or with Make
make build
```

### 2. Start in monitoring-only mode

```bash
# With Docker Compose
docker-compose up -d

# Or with Make
make run

# View logs
docker-compose logs -f ecsazrlc
# or
make logs
```

### 3. Start with ECS integration

Create a `.env` file from `.env.example`:

```bash
cp .env.example .env
```

Edit `.env`:
```ini
AWS_REGION=eu-west-1
ECS_CLUSTER_NAME=your-cluster
```

Start:
```bash
docker-compose run -d ecsazrlc --enable-ecs --cluster your-cluster --heartbeat 30s

# Or with Make
make run-ecs ECS_CLUSTER_NAME=your-cluster
```

## Local Build (without Docker)

### Linux/macOS

```bash
cd ecsazrlc
go mod download
go build -o ecsazrlc ./cmd

# Run
./ecsazrlc --monitor-only
```

### Windows

```bash
cd ecsazrlc
go mod download
go build -o ecsazrlc.exe ./cmd

# Run
.\ecsazrlc.exe --monitor-only
```

### With Make

```bash
# Linux/macOS
make build-local
make run-local

# Windows
make build-local-windows
cd ecsazrlc && .\ecsazrlc.exe --monitor-only
```

## Docker Compose Configuration Options

### Monitoring-only mode

```yaml
command:
  - "--monitor-only"
  - "--verbose"  # For detailed logs
```

### ECS mode

```yaml
command:
  - "--enable-ecs"
  - "--cluster"
  - "my-cluster"
  - "--heartbeat"
  - "30s"
environment:
  - AWS_REGION=eu-west-1
```

### With AWS credentials (development)

```yaml
environment:
  - AWS_ACCESS_KEY_ID=your-key
  - AWS_SECRET_ACCESS_KEY=your-secret
  - AWS_REGION=eu-west-1
```

### With credentials file

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock:ro
  - ~/.aws:/home/ecsazrlc/.aws:ro
environment:
  - AWS_PROFILE=default
  - AWS_REGION=eu-west-1
```

## Multi-architecture Build

### For Linux (from Windows/macOS)

```bash
docker buildx build \
  --platform linux/amd64 \
  -t ecsazrlc:latest \
  -f ecsazrlc/Dockerfile \
  ecsazrlc/
```

### For ARM (Graviton)

```bash
docker buildx build \
  --platform linux/arm64 \
  -t ecsazrlc:latest-arm64 \
  -f ecsazrlc/Dockerfile \
  ecsazrlc/
```

## EC2 Deployment

### 1. Docker Installation

```bash
# Amazon Linux 2
sudo yum update -y
sudo yum install -y docker
sudo systemctl start docker
sudo systemctl enable docker
sudo usermod -aG docker ec2-user
```

### 2. Clone the repository

```bash
git clone https://github.com/your-org/esc_azure_lifecircle.git
cd esc_azure_lifecircle
```

### 3. Build and start

```bash
# Monitoring-only mode
docker-compose up -d

# ECS mode (instance must have an IAM role)
docker-compose run -d ecsazrlc \
  --enable-ecs \
  --cluster prod-cluster \
  --heartbeat 30s
```

### 4. Verify

```bash
docker-compose ps
docker-compose logs -f ecsazrlc
```

### 5. Systemd service (optional)

Create `/etc/systemd/system/ecsazrlc.service`:

```ini
[Unit]
Description=ECS Azure Lifecircle Monitor
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/esc_azure_lifecircle
ExecStart=/usr/local/bin/docker-compose up -d
ExecStop=/usr/local/bin/docker-compose down
TimeoutStartSec=0

[Install]
WantedBy=multi-user.target
```

Enable:
```bash
sudo systemctl daemon-reload
sudo systemctl enable ecsazrlc
sudo systemctl start ecsazrlc
```

## ECS Deployment (Task Definition)

### 1. Push image to ECR

```bash
# ECR authentication
aws ecr get-login-password --region eu-west-1 | docker login --username AWS --password-stdin 123456789012.dkr.ecr.eu-west-1.amazonaws.com

# Tag and push
docker tag ecsazrlc:latest 123456789012.dkr.ecr.eu-west-1.amazonaws.com/ecsazrlc:latest
docker push 123456789012.dkr.ecr.eu-west-1.amazonaws.com/ecsazrlc:latest
```

### 2. Task Definition

Create `task-definition.json`:

```json
{
  "family": "ecsazrlc",
  "taskRoleArn": "arn:aws:iam::123456789012:role/ecsazrlc-task-role",
  "executionRoleArn": "arn:aws:iam::123456789012:role/ecsTaskExecutionRole",
  "networkMode": "bridge",
  "containerDefinitions": [
    {
      "name": "ecsazrlc",
      "image": "123456789012.dkr.ecr.eu-west-1.amazonaws.com/ecsazrlc:latest",
      "memory": 256,
      "cpu": 256,
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
          "containerPath": "/var/run/docker.sock",
          "readOnly": true
        }
      ],
      "command": [
        "--enable-ecs",
        "--cluster", "prod-cluster",
        "--heartbeat", "30s"
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/ecsazrlc",
          "awslogs-region": "eu-west-1",
          "awslogs-stream-prefix": "ecs"
        }
      }
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

### 3. Register and launch

```bash
# Register task definition
aws ecs register-task-definition --cli-input-json file://task-definition.json

# Run task
aws ecs run-task \
  --cluster prod-cluster \
  --task-definition ecsazrlc \
  --count 1
```

## Available Make Commands

```bash
make help              # Show help
make build             # Build Docker image
make run               # Start in monitoring mode
make run-ecs           # Start with ECS
make run-dev           # Verbose mode for development
make stop              # Stop services
make restart           # Restart
make logs              # View logs
make clean             # Clean up
make test              # Run tests
make build-local       # Build local binary
make run-local         # Run locally
make test-with-agent   # Test with Azure agent
```

## Troubleshooting

### Error: "permission denied" on Docker socket

```bash
# Add user to docker group
sudo usermod -aG docker $USER
# Then logout/login

# Or on Windows, verify Docker Desktop is running
```

### Error: "no such file or directory: /var/run/docker.sock"

On Windows, modify `docker-compose.yml`:
```yaml
volumes:
  - //var/run/docker.sock:/var/run/docker.sock:ro
  # or
  - /var/run/docker.sock:/var/run/docker.sock:ro
```

### Image won't build

```bash
# Check build logs
docker-compose build --no-cache --progress=plain

# Check go.mod
cd ecsazrlc && go mod tidy
```

### AWS credentials not working

See [CREDENTIALS.md](ecsazrlc/CREDENTIALS.md) for detailed configuration.

## Tests

### Local test with Docker Compose

```bash
# Start with test Azure agent
make test-with-agent

# Verify agent is detected
make logs
```

### Test on EC2

```bash
# SSH to instance
ssh ec2-user@instance-ip

# Verify IAM credentials
aws sts get-caller-identity

# Start monitoring
cd /opt/esc_azure_lifecircle
docker-compose up -d

# Follow logs
docker-compose logs -f
```
