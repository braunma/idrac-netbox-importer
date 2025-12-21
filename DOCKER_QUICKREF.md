# Docker & CI/CD Quick Reference

## Dockerfile Status ✅

### Fixed Issues
- ✅ **Go version updated**: 1.21 → 1.22 (matches go.mod)
- ✅ **Alpine version updated**: 3.19 → 3.21 (latest stable)

### Dockerfile Features
- ✅ Multi-stage build (optimized size: ~15-20 MB)
- ✅ Security hardened (non-root user, UID 1000)
- ✅ Health check enabled
- ✅ Build args for version injection
- ✅ Layer caching optimized
- ✅ CA certificates included
- ✅ Timezone data included

---

## Quick Commands

### Build

```bash
# Basic build
docker build -t idrac-inventory .

# With version info
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ') \
  --build-arg GIT_COMMIT=$(git rev-parse --short HEAD) \
  -t idrac-inventory:1.0.0 .

# Using Makefile
make docker
```

### Run

```bash
# Single host scan
docker run --rm \
  idrac-inventory:latest \
  -host idrac1.example.com -user root -pass calvin

# With config file
docker run --rm \
  -v $(pwd)/config.yaml:/app/config.yaml \
  idrac-inventory:latest \
  -config /app/config.yaml

# With NetBox sync
docker run --rm \
  -e NETBOX_URL=https://netbox.example.com \
  -e NETBOX_TOKEN=your-token \
  -v $(pwd)/config.yaml:/app/config.yaml \
  idrac-inventory:latest \
  -config /app/config.yaml -sync
```

### Push to Registry

```bash
# GitLab Container Registry
docker tag idrac-inventory:latest registry.gitlab.com/yourgroup/idrac-inventory:latest
docker push registry.gitlab.com/yourgroup/idrac-inventory:latest

# Docker Hub
docker tag idrac-inventory:latest yourusername/idrac-inventory:latest
docker push yourusername/idrac-inventory:latest
```

---

## GitLab CI/CD Pipeline

### Stages Overview

```
┌─────────────┐
│  validate   │ → Code quality, formatting, go vet, lint
└──────┬──────┘
       ↓
┌─────────────┐
│    test     │ → Unit tests, integration tests, coverage
└──────┬──────┘
       ↓
┌─────────────┐
│    build    │ → Binary build, Docker build, multi-arch
└──────┬──────┘
       ↓
┌─────────────┐
│  security   │ → Dependency scan, secret scan, container scan
└──────┬──────┘
       ↓
┌─────────────┐
│   release   │ → Create release, upload artifacts (tags only)
└──────┬──────┘
       ↓
┌─────────────┐
│   deploy    │ → Staging/production deploy (manual)
└─────────────┘
```

### Pipeline Triggers

| Event | Stages Run | Notes |
|-------|------------|-------|
| Push to branch | validate, test, build, security | Full pipeline except release |
| Push tag (v*) | All stages | Includes release creation |
| Merge request | validate, test | Quick feedback |
| Scheduled | security only | Nightly scans |
| Manual | deploy | Deployment jobs |

### Key Jobs

| Job | Stage | When | Artifacts |
|-----|-------|------|-----------|
| `code-format-check` | validate | Always | None |
| `unit-tests` | test | Always | coverage.out |
| `build-binary` | build | Always | Binary (1 week) |
| `build-multi-arch` | build | Tags/main | All binaries (1 month) |
| `docker-build` | build | Always | Image pushed to registry |
| `docker-build-release` | build | Tags only | Release image (latest) |
| `dependency-scanning` | security | Always | Vulnerability report |
| `create-release` | release | Tags only | GitLab release page |

---

## Environment Variables

### Docker Runtime

| Variable | Default | Description |
|----------|---------|-------------|
| `IDRAC_LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |
| `IDRAC_LOG_FORMAT` | `json` | Log format (json/console) |
| `IDRAC_DEFAULT_TIMEOUT` | `60` | Connection timeout (seconds) |
| `IDRAC_CONCURRENCY` | `10` | Max parallel scans |
| `IDRAC_DEFAULT_USER` | - | Default iDRAC username |
| `IDRAC_DEFAULT_PASS` | - | Default iDRAC password |
| `NETBOX_URL` | - | NetBox API URL |
| `NETBOX_TOKEN` | - | NetBox API token |

### GitLab CI/CD (Auto-set)

| Variable | Description |
|----------|-------------|
| `CI_COMMIT_TAG` | Tag name (e.g., v1.0.0) |
| `CI_COMMIT_SHORT_SHA` | Short commit SHA |
| `CI_REGISTRY` | GitLab container registry |
| `CI_REGISTRY_IMAGE` | Full image path |
| `CI_JOB_TOKEN` | Job authentication token |

---

## Files Created

### New Files
- ✅ `.gitlab-ci.yml` - Complete CI/CD pipeline
- ✅ `.dockerignore` - Docker build exclusions
- ✅ `CICD_GUIDE.md` - Comprehensive CI/CD documentation
- ✅ `DOCKER_QUICKREF.md` - This quick reference

### Modified Files
- ✅ `Dockerfile` - Updated Go 1.22, Alpine 3.21

---

## Common Tasks

### Create a Release

```bash
# 1. Update version
# Edit files if needed

# 2. Commit changes
git add .
git commit -m "Release v1.0.0"

# 3. Create and push tag
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0

# 4. Pipeline automatically:
#    - Runs all tests
#    - Builds multi-platform binaries
#    - Builds Docker images
#    - Creates GitLab release
#    - Uploads artifacts
```

### Local Testing

```bash
# Test Docker build
docker build -t test .

# Test application
docker run --rm test -version

# Test with real config (dry run)
docker run --rm \
  -v $(pwd)/config.yaml:/app/config.yaml \
  test -config /app/config.yaml -validate

# Run unit tests locally
make test-unit

# Run all checks
make check
```

### Deploy to Production

```bash
# 1. Create release tag (see above)

# 2. In GitLab UI:
#    - Go to CI/CD → Pipelines
#    - Find pipeline for your tag
#    - Click "Play" on deploy-production job
#    - Confirm deployment

# 3. Verify deployment
curl https://your-production-server/health
```

---

## Troubleshooting

### Pipeline Failures

**Lint fails**:
```bash
# Run locally
make lint

# Fix formatting
make fmt
```

**Tests fail**:
```bash
# Run locally
make test

# Run specific test
go test -v ./internal/scanner/... -run TestName
```

**Docker build fails**:
```bash
# Check Docker daemon
docker ps

# Clean build cache
docker builder prune

# Build locally
make docker
```

### Common Issues

| Issue | Solution |
|-------|----------|
| "go.mod: no such file" | Ensure you're in project root |
| "permission denied" | Check file permissions, run as non-root |
| "tag already exists" | Delete tag: `git tag -d v1.0.0 && git push origin :v1.0.0` |
| "cache miss" | Normal on first run, subsequent runs will be faster |
| "out of disk space" | Clean Docker: `docker system prune -a` |

---

## Security Checklist

Before releasing:

- [ ] All tests passing
- [ ] No hardcoded secrets in code
- [ ] Dependencies scanned for vulnerabilities
- [ ] Container scanned with Trivy
- [ ] CHANGELOG.md updated
- [ ] Version bumped in appropriate files
- [ ] Tag follows semantic versioning (v1.2.3)
- [ ] GitLab protected branches configured
- [ ] CI/CD variables are masked
- [ ] Production deployment requires manual approval

---

## Next Steps

1. **Customize deployment**
   - Edit `deploy-staging` and `deploy-production` jobs in `.gitlab-ci.yml`
   - Add your SSH keys to GitLab CI/CD variables
   - Configure your deployment targets

2. **Set up schedules**
   - GitLab → CI/CD → Schedules
   - Add nightly security scans
   - Add weekly dependency updates

3. **Configure notifications**
   - Add Slack webhook to CI/CD variables
   - Uncomment notification jobs in pipeline

4. **Add badges to README**
   ```markdown
   [![pipeline](https://gitlab.com/yourgroup/idrac-inventory/badges/main/pipeline.svg)](https://gitlab.com/yourgroup/idrac-inventory/-/commits/main)
   [![coverage](https://gitlab.com/yourgroup/idrac-inventory/badges/main/coverage.svg)](https://gitlab.com/yourgroup/idrac-inventory/-/commits/main)
   ```

---

## Resource Links

- **GitLab CI/CD Docs**: https://docs.gitlab.com/ee/ci/
- **Docker Best Practices**: https://docs.docker.com/develop/dev-best-practices/
- **Go Testing**: https://go.dev/doc/tutorial/add-a-test
- **Multi-arch Builds**: https://docs.docker.com/build/building/multi-platform/

---

## Summary

✅ **Dockerfile**: Production-ready, secure, optimized
✅ **CI/CD Pipeline**: Comprehensive, automated, tested
✅ **Documentation**: Complete guides and references
✅ **Security**: Scanning, hardening, best practices
✅ **Automation**: Build, test, release, deploy

**Status**: Ready for production use! 🚀
