# GitLab CI/CD Setup Guide

## Overview

This guide walks you through setting up the iDRAC NetBox Importer in your local GitLab instance with CI/CD pipelines.

---

## Prerequisites

- Local GitLab instance (self-hosted)
- GitLab Runner configured with Docker executor
- Access to iDRAC servers
- (Optional) NetBox instance for inventory synchronization

---

## Quick Start

### 1. Initial Setup

```bash
# Clone or create your GitLab repository
cd /path/to/idrac-netbox-importer

# Initialize git if not already done
git init
git add .
git commit -m "Initial commit"

# Add your GitLab remote
git remote add origin git@your-gitlab.com:your-group/idrac-netbox-importer.git

# Push to GitLab
git push -u origin main
```

### 2. Configure CI/CD Variables

In your GitLab project:

**Settings → CI/CD → Variables → Add variable**

**Required Variables:**

| Key | Value | Masked | Protected |
|-----|-------|--------|-----------|
| `IDRAC_DEFAULT_USER` | `root` (or your default) | ✓ | - |
| `IDRAC_DEFAULT_PASS` | Your iDRAC password | ✓ | ✓ |

**Optional Variables (for NetBox sync):**

| Key | Value | Masked | Protected |
|-----|-------|--------|-----------|
| `NETBOX_URL` | `https://netbox.yourdomain.com` | - | - |
| `NETBOX_TOKEN` | Your NetBox API token | ✓ | ✓ |

**Configuration Variables:**

| Key | Value | Description |
|-----|-------|-------------|
| `IDRAC_LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `IDRAC_CONCURRENCY` | `10` | Max parallel scans |

### 3. Enable Container Registry

**Settings → General → Visibility, project features, permissions**

- Ensure "Container Registry" is enabled

### 4. Configure GitLab Runner

Ensure your GitLab Runner has the Docker executor:

```bash
# Check runner status
gitlab-runner status

# Verify Docker executor in config
cat /etc/gitlab-runner/config.toml
```

Required runner configuration:
```toml
[[runners]]
  executor = "docker"
  [runners.docker]
    image = "golang:1.22-alpine"
    privileged = true
    volumes = ["/cache", "/var/run/docker.sock:/var/run/docker.sock"]
```

---

## Configuration Files

### config.yaml

Create your configuration file for iDRAC servers:

```yaml
# NetBox configuration (uses environment variables)
netbox:
  url: "${NETBOX_URL}"
  token: "${NETBOX_TOKEN}"
  insecure_skip_verify: false
  timeout_seconds: 30

# Default credentials (uses environment variables)
defaults:
  username: "${IDRAC_DEFAULT_USER}"
  password: "${IDRAC_DEFAULT_PASS}"
  timeout_seconds: 60
  insecure_skip_verify: true

# Parallel scanning
concurrency: 10

# Logging
logging:
  level: info
  format: json

# Servers to scan
servers:
  - host: idrac1.yourdomain.com
    name: "Server 1"

  - host: idrac2.yourdomain.com
    name: "Server 2"

  - host: 192.168.1.10
    name: "Server 3"
    # Override default credentials for specific servers
    username: custom-user
    password: custom-pass
```

**Important:** Use environment variable substitution `${VAR_NAME}` for sensitive values.

---

## Running Locally with Docker

### Using Docker Compose

```bash
# 1. Copy environment template
cp .env.example .env

# 2. Edit .env with your credentials
nano .env

# 3. Run
docker-compose up

# 4. Run with NetBox sync
docker-compose run idrac-scanner -config /app/config.yaml -sync
```

### Using Docker Directly

```bash
# Build image
docker build -t idrac-inventory:local .

# Run scan
docker run --rm \
  -e IDRAC_DEFAULT_USER=root \
  -e IDRAC_DEFAULT_PASS=your-password \
  -v $(pwd)/config.yaml:/app/config.yaml \
  idrac-inventory:local -config /app/config.yaml -verbose

# Run with NetBox sync
docker run --rm \
  -e IDRAC_DEFAULT_USER=root \
  -e IDRAC_DEFAULT_PASS=your-password \
  -e NETBOX_URL=https://netbox.yourdomain.com \
  -e NETBOX_TOKEN=your-token \
  -v $(pwd)/config.yaml:/app/config.yaml \
  idrac-inventory:local -config /app/config.yaml -sync
```

---

## GitLab CI/CD Pipeline

### Pipeline Overview

The pipeline runs automatically on every commit:

```
validate → test → build → security → release → deploy
```

**Stages:**

1. **Validate** - Code formatting, linting, go vet
2. **Test** - Unit tests, integration tests, coverage
3. **Build** - Binary compilation, Docker image build
4. **Security** - Dependency scanning, container scanning
5. **Release** - Create GitLab releases (tags only)
6. **Deploy** - Deploy to staging/production (manual)

### Triggering the Pipeline

**Automatic triggers:**
- Push to any branch → Runs validate, test, build, security
- Push a tag (e.g., `v1.0.0`) → Full pipeline including release
- Merge request → Quick validation and tests

**Manual triggers:**
- Multi-arch Docker build
- Deployment jobs

### Creating a Release

```bash
# 1. Commit your changes
git add .
git commit -m "Release v1.0.0"

# 2. Create and push a tag
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0

# 3. Pipeline automatically:
#    - Runs all tests
#    - Builds binaries for all platforms
#    - Builds and tags Docker image as v1.0.0 and latest
#    - Creates GitLab release page
#    - Uploads artifacts
```

### Viewing Pipeline Results

**Navigate to:** CI/CD → Pipelines

Each pipeline shows:
- ✓ Job status (passed/failed)
- 📊 Code coverage percentage
- 🐳 Docker image tags
- 📦 Downloadable artifacts

---

## Using the Built Images

### From GitLab Container Registry

```bash
# Login to your GitLab registry
docker login your-gitlab.com:5050

# Pull latest image
docker pull your-gitlab.com:5050/your-group/idrac-netbox-importer:latest

# Pull specific version
docker pull your-gitlab.com:5050/your-group/idrac-netbox-importer:v1.0.0

# Pull by commit SHA
docker pull your-gitlab.com:5050/your-group/idrac-netbox-importer:abc123

# Run
docker run --rm \
  -e IDRAC_DEFAULT_USER=root \
  -e IDRAC_DEFAULT_PASS=password \
  your-gitlab.com:5050/your-group/idrac-netbox-importer:latest \
  -config /app/config.yaml
```

### Scheduled Scans with GitLab CI/CD

Create a scheduled pipeline for nightly scans:

**CI/CD → Schedules → New schedule**

- Description: "Nightly inventory scan"
- Interval: `0 2 * * *` (2 AM daily)
- Target branch: `main`
- Variables: (none needed, uses project variables)

Add a custom job in `.gitlab-ci.yml`:

```yaml
nightly-scan:
  stage: deploy
  image: $CI_REGISTRY_IMAGE:latest
  script:
    - /app/idrac-inventory -config /app/config.yaml -sync -output json > scan-results.json
  artifacts:
    paths:
      - scan-results.json
    expire_in: 30 days
  only:
    - schedules
```

---

## Deployment

### Manual Deployment to Staging

1. Go to: **CI/CD → Pipelines**
2. Select the pipeline for `main` branch
3. Find `deploy-staging` job
4. Click "Play" button to trigger

### Manual Deployment to Production

1. Create a release tag (e.g., `v1.0.0`)
2. Go to: **CI/CD → Pipelines**
3. Select the pipeline for your tag
4. Find `deploy-production` job
5. Click "Play" button to trigger

### Customizing Deployment

Edit `.gitlab-ci.yml` deployment jobs with your actual deployment commands:

```yaml
deploy-production:
  stage: deploy
  image: alpine:3.21
  before_script:
    - apk add --no-cache openssh-client
    - eval $(ssh-agent -s)
    - echo "$PRODUCTION_SSH_KEY" | tr -d '\r' | ssh-add -
  script:
    # Your deployment commands here
    - ssh user@your-server "docker pull $CI_REGISTRY_IMAGE:$CI_COMMIT_TAG"
    - ssh user@your-server "docker-compose -f /app/docker-compose.yml up -d"
  environment:
    name: production
  only:
    - tags
  when: manual
```

---

## Monitoring and Logs

### Pipeline Logs

**CI/CD → Pipelines → Select pipeline → Select job**

View real-time logs and download job artifacts.

### Container Logs

```bash
# View logs from running container
docker logs idrac-scanner

# Follow logs
docker logs -f idrac-scanner

# View last 100 lines
docker logs --tail 100 idrac-scanner
```

### Health Checks

```bash
# Check container health
docker ps

# Check application version
docker exec idrac-scanner /app/idrac-inventory -version
```

---

## Troubleshooting

### Pipeline Fails: "Cannot connect to Docker daemon"

**Solution:**
1. Ensure GitLab Runner has Docker executor enabled
2. Check runner has access to Docker socket
3. Verify `privileged = true` in runner config

### Pipeline Fails: "go: downloading timeout"

**Solution:**
1. Check GitLab Runner has internet access
2. Verify firewall allows Go module downloads
3. Add `GOPROXY=https://proxy.golang.org,direct` to variables

### Docker Build Fails: "permission denied"

**Solution:**
1. Ensure user is in `docker` group: `usermod -aG docker gitlab-runner`
2. Restart GitLab Runner: `systemctl restart gitlab-runner`
3. Check Docker socket permissions

### Variables Not Available in Job

**Solution:**
1. Ensure variables are set at project level (not group)
2. Check variable is not "Protected" if running on non-protected branch
3. Verify variable name matches exactly (case-sensitive)

### Image Push Fails: "unauthorized"

**Solution:**
1. Registry must be enabled in project settings
2. Ensure `$CI_REGISTRY_PASSWORD` is available (automatic in GitLab)
3. Check runner can access GitLab registry

---

## Security Best Practices

### Variable Security

✅ **DO:**
- Mark all passwords/tokens as "Masked"
- Mark production variables as "Protected"
- Use environment variable substitution in config files
- Rotate secrets regularly

❌ **DON'T:**
- Commit `.env` files
- Hardcode credentials in `config.yaml`
- Print sensitive variables in job logs
- Share tokens in plain text

### Branch Protection

**Settings → Repository → Protected Branches**

For `main` branch:
- ✓ Allowed to merge: Maintainers
- ✓ Allowed to push: No one
- ✓ Require code owner approval
- ✓ Pipelines must succeed

### Container Security

The Docker image is built with security in mind:
- ✓ Multi-stage build (minimal size)
- ✓ Non-root user (UID 1000)
- ✓ No unnecessary packages
- ✓ Security scanning in CI/CD
- ✓ Health checks enabled

---

## Advanced Configuration

### Custom NetBox Fields

If your NetBox uses different field names:

**Settings → CI/CD → Variables**

```
NETBOX_FIELD_CPU_COUNT=my_cpu_count
NETBOX_FIELD_CPU_MODEL=my_cpu_model
NETBOX_FIELD_RAM_TOTAL=my_ram_total
# ... etc
```

See `.gitlab-ci-variables.example.yml` for complete list.

### Multi-Environment Setup

Use different variables for different environments:

**Development:**
```
IDRAC_DEFAULT_USER (dev branch) = readonly_user
NETBOX_URL (dev branch) = https://netbox-dev.yourdomain.com
```

**Production:**
```
IDRAC_DEFAULT_USER (main/tags) = admin_user
NETBOX_URL (main/tags) = https://netbox.yourdomain.com
```

---

## Getting Help

### Common Commands

```bash
# View pipeline status
gitlab-ci-multi-runner status

# Test job locally
gitlab-runner exec docker unit-tests

# Validate .gitlab-ci.yml
gitlab-ci-lint .gitlab-ci.yml

# Check Docker images
docker images | grep idrac-inventory

# Clean up old images
docker image prune -a
```

### Useful Links

- GitLab CI/CD: https://docs.gitlab.com/ee/ci/
- Docker Documentation: https://docs.docker.com/
- Go Modules: https://go.dev/ref/mod

---

## Summary

You now have:
- ✅ GitLab repository with CI/CD pipeline
- ✅ Automated building and testing
- ✅ Docker images in GitLab Registry
- ✅ Secure variable management
- ✅ Deployment automation

**Next steps:**
1. Customize `config.yaml` with your servers
2. Set up CI/CD variables in GitLab
3. Push code and watch pipeline run
4. Create first release tag
5. Schedule nightly scans

All credentials are managed through GitLab CI/CD variables - no hardcoded secrets! 🔒
