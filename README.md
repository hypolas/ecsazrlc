# ECS Azure Lifecycle Monitor (ecsazrlc)

[![Docker Hub](https://img.shields.io/badge/docker-hypolas%2Fecsazrlc-blue?logo=docker)](https://hub.docker.com/r/hypolas/ecsazrlc)
[![Docker Pulls](https://img.shields.io/docker/pulls/hypolas/ecsazrlc)](https://hub.docker.com/r/hypolas/ecsazrlc)
[![Go Version](https://img.shields.io/badge/go-1.25.1+-00ADD8?logo=go)](https://go.dev/)

> **AWS ECS Instance Protection** | **Azure DevOps Agent Monitor** | **Docker Container Activity Tracker** | **CI/CD Build Protection**

Monitor Azure DevOps agents running in Docker containers and communicate with AWS ECS to prevent premature instance termination during active builds.

**Docker Hub**: https://hub.docker.com/r/hypolas/ecsazrlc

## Overview

This tool watches Docker socket for Azure DevOps Agent container activity and informs AWS ECS about server activity, preventing termination of instances with running builds.

**Keywords**: AWS ECS, Azure DevOps, Azure Pipelines, Docker monitoring, instance lifecycle, CI/CD, container monitoring, build agent protection, AWS auto-scaling, spot instance protection

## Key Features

- **Real-time Docker monitoring** - Listens to Docker socket events
- **Azure agent detection** - Automatically identifies Azure DevOps Agent containers
- **ECS heartbeat** - Sends periodic activity signals to ECS
- **Instance protection** - Can enable/disable termination protection
- **Standalone mode** - Can run in monitoring-only mode without ECS

## Quick Start

### Pull from Docker Hub

```bash
# Pull the latest image
docker pull hypolas/ecsazrlc:latest

# Run in monitoring-only mode
docker run -v /var/run/docker.sock:/var/run/docker.sock:ro hypolas/ecsazrlc:latest --monitor-only

# Run with ECS integration
docker run -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -e AWS_REGION=us-east-1 \
  hypolas/ecsazrlc:latest --enable-ecs --cluster my-cluster
```

### With Docker Compose

Create a `docker-compose.yml`:

```yaml
services:
  ecsazrlc:
    image: hypolas/ecsazrlc:latest
    container_name: ecsazrlc
    restart: unless-stopped

    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro

    environment:
      - AWS_REGION=us-east-1
      # Optional: for local dev/testing
      # - AWS_ACCESS_KEY_ID=your-key
      # - AWS_SECRET_ACCESS_KEY=your-secret

    command:
      - "--monitor-only"
      # For ECS integration, use:
      # - "--enable-ecs"
      # - "--cluster"
      # - "your-cluster-name"
      # - "--heartbeat"
      # - "30s"
```

Then run:

```bash
# Start
docker-compose up -d

# View logs
docker-compose logs -f ecsazrlc

# Stop
docker-compose down
```

### With Podman Compose

```bash
# Build and start
podman compose up -d

# View logs
podman compose logs -f ecsazrlc
```

### Local Build

```bash
go build -o ecsazrlc ./cmd
./ecsazrlc --monitor-only
```

## Usage

### Monitoring only (no ECS)

```bash
./ecsazrlc --monitor-only --verbose
```

### With ECS integration

```bash
./ecsazrlc --enable-ecs --cluster my-cluster --heartbeat 30s
```

## Use Cases

- **Prevent build interruption**: Protect EC2/ECS instances running Azure DevOps agents from termination during active builds
- **Cost optimization**: Use AWS spot instances or auto-scaling for CI/CD without losing running jobs
- **Hybrid CI/CD**: Run Azure DevOps agents on AWS ECS infrastructure
- **Container lifecycle management**: Monitor Docker container activity for custom automation
- **Multi-cloud CI/CD**: Bridge Azure DevOps with AWS compute resources

## Documentation

- **[BUILD.md](BUILD.md)** - Complete build and deployment guide
  - Docker Compose setup
  - Podman support
  - Local builds (Linux/macOS/Windows)
  - Multi-architecture builds (amd64/arm64)
  - EC2 deployment
  - ECS Task Definition examples

- **[CREDENTIALS.md](CREDENTIALS.md)** - AWS credentials configuration
  - IAM roles (recommended for production)
  - Environment variables
  - Credentials file
  - Required IAM permissions

- **[TESTING.md](TESTING.md)** - Testing guide
  - Local testing with Docker Compose
  - EC2 testing
  - Azure agent simulation

## Architecture

```
┌─────────────────────┐         ┌──────────────────┐
│  Docker Socket      │────────▶│   ecsazrlc       │
│  (container events) │         │   (monitor)      │
└─────────────────────┘         └────────┬─────────┘
                                         │
                                ┌────────▼─────────┐
                                │   AWS ECS API    │
                                │   (heartbeat)    │
                                └──────────────────┘
```

## Technology Stack

- **Language**: Go 1.25.1+
- **Cloud**: AWS ECS, EC2, IAM
- **CI/CD**: Azure DevOps, Azure Pipelines
- **Container**: Docker, Podman
- **SDK**: AWS SDK for Go v2, Docker Engine API

## Requirements

- Docker or Podman
- Go 1.25.1+ (for local builds)
- AWS credentials (IAM role recommended)
- Access to Docker socket

## AWS Permissions

The EC2/ECS instance needs:

- `ecs:DescribeClusters`
- `ecs:ListContainerInstances`
- `ecs:DescribeContainerInstances`
- `ecs:PutAttributes`
- `ecs:UpdateContainerInstancesState`

See [CREDENTIALS.md](CREDENTIALS.md) for details.

## Environment Variables

- `AWS_REGION` - AWS region (default: us-east-1)
- `AWS_DEFAULT_REGION` - Alternative AWS region
- `AWS_PROFILE` - AWS profile to use (default: default)
- `DOCKER_HOST` - Docker socket (default: unix:///var/run/docker.sock)

## Command-line Options

- `--cluster` - ECS cluster name (required in ECS mode)
- `--heartbeat` - Heartbeat interval (default: 30s)
- `--enable-ecs` - Enable ECS notifications
- `--monitor-only` - Monitoring-only mode without ECS
- `--verbose` - Verbose mode with detailed logs

## Supported Platforms

- Linux (amd64, arm64)
- Windows (amd64)
- macOS (amd64, arm64)
- Docker containers
- Podman containers
- AWS EC2 instances
- AWS ECS tasks

## Integration Examples

### AWS ECS with Azure DevOps

Run Azure Pipelines agents on AWS ECS infrastructure with automatic lifecycle management.

### Spot Instance Protection

Use AWS spot instances for cost savings while ensuring running builds are never interrupted.

### Auto-scaling CI/CD

Scale Azure DevOps agent pools on AWS with ECS auto-scaling while protecting active build agents.

## Contributing

Contributions welcome! Please open an issue or pull request.

## License

MIT

## Tags

`aws-ecs` `azure-devops` `azure-pipelines` `docker-monitoring` `ci-cd` `golang` `container-lifecycle` `instance-protection` `devops-automation` `hybrid-cloud` `spot-instances` `auto-scaling` `build-agents` `docker-events` `ecs-heartbeat`
