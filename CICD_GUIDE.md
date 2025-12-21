# CI/CD & Docker Guide

## Overview

This project includes a comprehensive GitLab CI/CD pipeline and optimized Docker configuration for automated building, testing, and deployment.

---

## Table of Contents

- [Docker Configuration](#docker-configuration)
- [GitLab CI/CD Pipeline](#gitlab-cicd-pipeline)
- [Pipeline Stages](#pipeline-stages)
- [Environment Variables](#environment-variables)
- [GitLab Setup](#gitlab-setup)
- [Usage Examples](#usage-examples)
- [Troubleshooting](#troubleshooting)

---

## Docker Configuration

### Dockerfile Features

✅ **Multi-stage build** - Optimized image size (build stage + runtime stage)
✅ **Security hardened** - Non-root user (UID 1000)
✅ **Latest versions** - Go 1.22, Alpine 3.21
✅ **Health check** - Automatic container health monitoring
✅ **Layer caching** - Optimized for fast rebuilds
✅ **Build arguments** - Version info injection via ldflags
✅ **Minimal runtime** - Only essential dependencies in final image

### Building the Docker Image

```bash
# Basic build
docker build -t idrac-inventory .

# Build with version information
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ') \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  -t idrac-inventory:1.0.0 \
  .

# Using Makefile
make docker
```

### Running the Docker Container

```bash
# Run with config file
docker run --rm \
  -e IDRAC_DEFAULT_USER=admin \
  -e IDRAC_DEFAULT_PASS=secret \
  -v $(pwd)/config.yaml:/app/config.yaml \
  idrac-inventory:latest -config /app/config.yaml

# Run against single host
docker run --rm \
  -e IDRAC_DEFAULT_USER=admin \
  -e IDRAC_DEFAULT_PASS=secret \
  idrac-inventory:latest \
  -host idrac1.example.com -user root -pass calvin

# Run with NetBox sync
docker run --rm \
  -e IDRAC_DEFAULT_USER=admin \
  -e IDRAC_DEFAULT_PASS=secret \
  -e NETBOX_URL=https://netbox.example.com \
  -e NETBOX_TOKEN=your-token-here \
  -v $(pwd)/config.yaml:/app/config.yaml \
  idrac-inventory:latest -config /app/config.yaml -sync

# Run in background (scheduled scans)
docker run -d \
  --name idrac-scanner \
  -e IDRAC_DEFAULT_USER=admin \
  -e IDRAC_DEFAULT_PASS=secret \
  -v $(pwd)/config.yaml:/app/config.yaml \
  idrac-inventory:latest -config /app/config.yaml
```

### Docker Compose Example

```yaml
version: '3.8'

services:
  idrac-scanner:
    image: registry.gitlab.com/yourgroup/idrac-inventory:latest
    container_name: idrac-scanner
    environment:
      - IDRAC_DEFAULT_USER=${IDRAC_USER}
      - IDRAC_DEFAULT_PASS=${IDRAC_PASS}
      - NETBOX_URL=${NETBOX_URL}
      - NETBOX_TOKEN=${NETBOX_TOKEN}
      - IDRAC_LOG_LEVEL=info
      - IDRAC_LOG_FORMAT=json
      - IDRAC_CONCURRENCY=10
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./logs:/app/logs
    command: -config /app/config.yaml -sync -output json
    restart: unless-stopped
    networks:
      - monitoring

networks:
  monitoring:
    driver: bridge
```

### Image Size Comparison

```
# Multi-stage build (optimized)
idrac-inventory:latest    ~15-20 MB

# If built without multi-stage
idrac-inventory:large     ~300+ MB
```

---

## GitLab CI/CD Pipeline

### Pipeline Architecture

The pipeline consists of **6 stages**:

```
validate → test → build → security → release → deploy
```

### Features

✅ **Automated testing** - Unit & integration tests with coverage reporting
✅ **Code quality** - Linting, formatting checks, go vet
✅ **Security scanning** - Dependency vulnerabilities, secret detection, container scanning
✅ **Multi-arch builds** - Linux (amd64, arm64), macOS, Windows
✅ **Docker builds** - Automatic image building and pushing to GitLab Registry
✅ **Automated releases** - GitHub-style releases with artifacts
✅ **Deployment automation** - Staging and production deployment (manual trigger)
✅ **Caching** - Go build cache for faster pipelines
✅ **Parallel execution** - Jobs run concurrently where possible

---

## Pipeline Stages

### 1. Validate Stage

**Purpose**: Ensure code quality and formatting

**Jobs**:
- `code-format-check` - Ensures code is properly formatted
- `go-vet` - Static analysis of Go code
- `go-mod-verify` - Verifies Go module integrity
- `lint` - Comprehensive linting with golangci-lint

**When it runs**: Every commit, every branch

**Failure behavior**: Blocks pipeline (except lint which is warning-only)

### 2. Test Stage

**Purpose**: Run tests and verify compilation

**Jobs**:
- `unit-tests` - Runs all unit tests with race detection and coverage
- `integration-tests` - Runs integration tests
- `build-test` - Verifies the binary compiles correctly

**Coverage Requirements**:
- Warning if coverage < 40%
- Coverage reports uploaded to GitLab

**When it runs**: Every commit, every branch

### 3. Build Stage

**Purpose**: Build binaries and Docker images

**Jobs**:
- `build-binary` - Builds single binary for current platform
- `build-multi-arch` - Builds for all platforms (tags/main only)
- `docker-build` - Builds and pushes Docker image
- `docker-build-release` - Builds release Docker image (tags only)
- `docker-buildx` - Multi-architecture Docker build (manual)

**Artifacts**:
- Binaries stored for 1 week (branches) or 1 month (tags)
- Docker images pushed to GitLab Container Registry

**When it runs**: Every commit (binary), tags for multi-arch

### 4. Security Stage

**Purpose**: Scan for vulnerabilities and secrets

**Jobs**:
- `dependency-scanning` - Scans Go dependencies for known vulnerabilities
- `secret-scanning` - Checks for hardcoded secrets in code
- `container-scanning` - Scans Docker images with Trivy

**When it runs**: Every commit, every branch

**Failure behavior**: Warning-only (doesn't block pipeline)

### 5. Release Stage

**Purpose**: Create releases and upload artifacts

**Jobs**:
- `create-release` - Creates GitLab release with changelog
- `upload-packages` - Uploads binaries to GitLab Package Registry

**When it runs**: Only on tags (e.g., v1.0.0)

**Release includes**:
- Version-tagged Docker image
- Multi-platform binaries
- Changelog
- Download links

### 6. Deploy Stage

**Purpose**: Deploy to environments

**Jobs**:
- `deploy-staging` - Deploy to staging (main branch, manual)
- `deploy-production` - Deploy to production (tags only, manual)

**When it runs**: Manual trigger only

**Note**: You need to customize these jobs with your actual deployment commands

---

## Environment Variables

### Required GitLab CI/CD Variables

Set these in GitLab: **Settings → CI/CD → Variables**

#### Docker Registry (Automatic)
These are automatically available in GitLab:
- `CI_REGISTRY` - GitLab container registry URL
- `CI_REGISTRY_USER` - Registry username
- `CI_REGISTRY_PASSWORD` - Registry password
- `CI_REGISTRY_IMAGE` - Full image path

#### Custom Variables (Optional)

**For deployment**:
- `STAGING_SSH_KEY` - SSH private key for staging server
- `PRODUCTION_SSH_KEY` - SSH private key for production server
- `STAGING_HOST` - Staging server hostname
- `PRODUCTION_HOST` - Production server hostname

**For external registries** (if not using GitLab Registry):
- `DOCKER_HUB_USERNAME` - Docker Hub username
- `DOCKER_HUB_TOKEN` - Docker Hub access token

**For notifications**:
- `SLACK_WEBHOOK_URL` - Slack webhook for notifications

### Pipeline Variables

These are automatically set by GitLab:
- `CI_COMMIT_TAG` - Tag name (if tagged commit)
- `CI_COMMIT_SHORT_SHA` - Short commit SHA
- `CI_COMMIT_REF_SLUG` - Branch name (sanitized)
- `CI_PIPELINE_CREATED_AT` - Pipeline creation timestamp
- `CI_PROJECT_ID` - GitLab project ID
- `CI_PROJECT_PATH` - Project path (group/project)

---

## GitLab Setup

### Initial Setup

1. **Push code to GitLab**
   ```bash
   git remote add origin git@gitlab.com:yourgroup/idrac-inventory.git
   git push -u origin main
   ```

2. **Enable Container Registry**
   - Go to: Settings → General → Visibility
   - Ensure "Container Registry" is enabled

3. **Configure CI/CD Variables**
   - Go to: Settings → CI/CD → Variables
   - Add any required secrets (SSH keys, tokens, etc.)

4. **Enable Pipeline**
   - Pipelines are automatically enabled when `.gitlab-ci.yml` is present
   - First pipeline triggers on next commit

### Creating a Release

```bash
# Create and push a tag
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0

# Pipeline automatically:
# 1. Runs all tests
# 2. Builds multi-arch binaries
# 3. Builds and pushes Docker image
# 4. Creates GitLab release
# 5. Uploads artifacts
```

### Scheduled Pipelines

Set up nightly security scans:

1. Go to: CI/CD → Schedules
2. Click "New schedule"
3. Set:
   - Description: "Nightly security scan"
   - Interval pattern: `0 2 * * *` (2 AM daily)
   - Target branch: `main`
   - Variables: None needed

---

## Usage Examples

### Running Pipeline Locally (for testing)

```bash
# Install gitlab-runner
# On macOS
brew install gitlab-runner

# On Linux
curl -L https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh | sudo bash
sudo apt-get install gitlab-runner

# Run specific job locally
gitlab-runner exec docker unit-tests

# Run with environment variables
gitlab-runner exec docker \
  --env GO_VERSION=1.22 \
  unit-tests
```

### Triggering Manual Jobs

Some jobs require manual triggering:

1. **Multi-arch Docker build**
   - Go to: CI/CD → Pipelines → Select pipeline
   - Find `docker-buildx` job
   - Click "Play" button

2. **Deployment**
   - Go to: CI/CD → Pipelines → Select pipeline
   - Find `deploy-staging` or `deploy-production`
   - Click "Play" button

### Viewing Pipeline Status

**Pipeline Badge**:
Add to README.md:
```markdown
[![pipeline status](https://gitlab.com/yourgroup/idrac-inventory/badges/main/pipeline.svg)](https://gitlab.com/yourgroup/idrac-inventory/-/commits/main)
```

**Coverage Badge**:
```markdown
[![coverage report](https://gitlab.com/yourgroup/idrac-inventory/badges/main/coverage.svg)](https://gitlab.com/yourgroup/idrac-inventory/-/commits/main)
```

---

## Troubleshooting

### Pipeline Fails on First Run

**Problem**: "No such file or directory: .cache/go-mod"

**Solution**: This is normal on first run. The cache will be created. Re-run the failed job.

### Docker Build Fails

**Problem**: "Cannot connect to Docker daemon"

**Solution**:
1. Ensure Docker-in-Docker service is configured
2. Check GitLab Runner has Docker executor
3. Verify DOCKER_TLS_CERTDIR is set

### Go Module Download Fails

**Problem**: "go: downloading github.com/... timeout"

**Solution**:
1. Check GitLab Runner has internet access
2. Use Go module proxy: Add `GOPROXY=https://proxy.golang.org,direct`
3. Ensure firewall allows Go module downloads

### Coverage Report Not Showing

**Problem**: Coverage percentage not displayed in GitLab

**Solution**:
1. Ensure `coverage: '/total:\s+\(statements\)\s+(\d+\.\d+)%/'` is in job
2. Check coverage.out artifact is uploaded
3. Verify regex matches your `go tool cover` output

### Multi-arch Build Fails

**Problem**: "exec format error" when building ARM image

**Solution**:
1. Ensure qemu-user-static is installed: `docker run --rm --privileged multiarch/qemu-user-static --reset -p yes`
2. Use buildx: The pipeline already does this in `docker-buildx` job
3. Run the job on a runner with multi-arch support

### Deployment Job Doesn't Run

**Problem**: Deployment job is skipped

**Solution**:
1. Check the `only:` conditions match (tags for production, main for staging)
2. Jobs are set to `when: manual` - you must trigger them manually
3. Ensure you're on the correct branch/tag

---

## Performance Optimization

### Pipeline Speed

**Current average times**:
- Validate stage: ~1-2 minutes
- Test stage: ~2-3 minutes
- Build stage: ~3-5 minutes
- Security stage: ~2-4 minutes
- **Total (typical)**: ~10-15 minutes

**Optimization tips**:
1. **Cache hit** - Go cache reduces test time by 50%
2. **Parallel jobs** - Multiple jobs run concurrently
3. **Docker layer cache** - `--cache-from` speeds up Docker builds
4. **Artifact reuse** - Build artifacts shared between jobs

### Resource Usage

**Runner requirements**:
- **CPU**: 2+ cores recommended
- **RAM**: 4GB minimum, 8GB recommended
- **Disk**: 20GB free space for Docker builds
- **Network**: Fast internet for Go module downloads

---

## Security Best Practices

### Secrets Management

✅ **DO**:
- Store secrets in GitLab CI/CD Variables (Settings → CI/CD → Variables)
- Mark sensitive variables as "Masked" and "Protected"
- Use `$CI_JOB_TOKEN` for GitLab API access
- Rotate secrets regularly

❌ **DON'T**:
- Hardcode secrets in .gitlab-ci.yml
- Print secrets in job logs
- Commit .env files to repository

### Container Security

✅ **Implemented**:
- Non-root user in Docker container
- Minimal base image (Alpine)
- Security scanning with Trivy
- Dependency vulnerability scanning
- No unnecessary packages in final image

### Branch Protection

Recommended GitLab settings:
1. **Settings → Repository → Protected Branches**
   - Protect `main` branch
   - Require merge request approval
   - Require pipeline success

2. **Settings → Merge Requests**
   - Enable "Pipelines must succeed"
   - Enable "All discussions must be resolved"

---

## Advanced Configurations

### Custom Deployment

Replace the example deployment jobs with your actual deployment:

```yaml
deploy-production:
  stage: deploy
  image: alpine:3.21
  before_script:
    - apk add --no-cache openssh-client
    - eval $(ssh-agent -s)
    - echo "$PRODUCTION_SSH_KEY" | tr -d '\r' | ssh-add -
    - mkdir -p ~/.ssh
    - chmod 700 ~/.ssh
  script:
    # Pull latest image
    - ssh user@$PRODUCTION_HOST "docker pull $IMAGE_NAME:$CI_COMMIT_TAG"
    # Run database migrations
    - ssh user@$PRODUCTION_HOST "cd /app && ./migrate.sh"
    # Rolling update
    - ssh user@$PRODUCTION_HOST "docker-compose up -d --no-deps --build app"
    # Health check
    - ssh user@$PRODUCTION_HOST "curl -f http://localhost:8080/health || exit 1"
  environment:
    name: production
    url: https://idrac.example.com
  only:
    - tags
  when: manual
```

### Notifications

Add Slack notifications:

```yaml
notify-slack:
  stage: .post
  image: curlimages/curl:latest
  script:
    - |
      curl -X POST -H 'Content-type: application/json' \
        --data "{\"text\":\"Pipeline $CI_PIPELINE_STATUS: $CI_PROJECT_NAME ($CI_COMMIT_REF_NAME)\"}" \
        $SLACK_WEBHOOK_URL
  when: on_failure
```

### Matrix Builds

Test multiple Go versions:

```yaml
unit-tests:
  stage: test
  extends: .go-base
  parallel:
    matrix:
      - GO_VERSION: ["1.21", "1.22", "1.23"]
  image: golang:${GO_VERSION}-alpine
  script:
    - go test -v ./...
```

---

## Conclusion

This CI/CD setup provides:
- ✅ Automated quality checks
- ✅ Comprehensive testing
- ✅ Security scanning
- ✅ Multi-platform builds
- ✅ Automated releases
- ✅ Deployment automation

**Next Steps**:
1. Customize deployment jobs for your environment
2. Set up pipeline schedules for nightly scans
3. Configure notifications
4. Add custom quality gates if needed

For issues or questions, check the GitLab CI/CD documentation: https://docs.gitlab.com/ee/ci/
