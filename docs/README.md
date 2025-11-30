# GiraffeCloud Documentation

This directory contains essential documentation for the GiraffeCloud secure tunnel service.

## 📚 Table of Contents

### 🚀 Getting Started

- **[installation.md](installation.md)** - CLI installation guide for macOS, Linux, and Windows

### 🏗️ Architecture

- **[dual-stream-architecture.md](dual-stream-architecture.md)** - Dual-stream control channel for instant cancellation
- **[hybrid-tunnel-architecture.md](hybrid-tunnel-architecture.md)** - Production-grade gRPC + TCP tunnel architecture
- **[keepalive-safety-mechanisms.md](keepalive-safety-mechanisms.md)** - Connection keepalive implementation
- **[grpc-large-file-handling.md](grpc-large-file-handling.md)** - Large file handling in gRPC

### 🎯 Features

- **[subdomain-feature.md](subdomain-feature.md)** - Auto-generated subdomain feature
- **[client-versioning.md](client-versioning.md)** - CLI client auto-update system
- **[contact-form-setup.md](contact-form-setup.md)** - Contact form configuration

### ⚙️ Deployment

- **[vps-deployment.md](vps-deployment.md)** - VPS deployment guide

## 📖 Documentation Organization

### Architecture Documentation
Core system architecture and technical implementation details.

### Feature Documentation
Guides for specific features and capabilities.

### Deployment Documentation
Production deployment guide.

## 🔗 Quick Links

### For End Users
1. Start with [installation.md](installation.md) to install the CLI
2. Review [subdomain-feature.md](subdomain-feature.md) for free subdomain usage
3. Check [client-versioning.md](client-versioning.md) for auto-update information

### For Developers
1. Understand the [hybrid-tunnel-architecture.md](hybrid-tunnel-architecture.md)
2. Review [dual-stream-architecture.md](dual-stream-architecture.md) for control channel details
3. Check [grpc-large-file-handling.md](grpc-large-file-handling.md) for large file optimization

### For DevOps
1. Review [vps-deployment.md](vps-deployment.md) for deployment
2. Check [keepalive-safety-mechanisms.md](keepalive-safety-mechanisms.md) for connection management


## 🗂️ Recent Cleanup (2025-11)

The following documentation was removed as no longer relevant:

### Phase 1: Automated Cleanup (16 files)
Implementation notes and bug fix documentation:
- `launch-readiness-2025-08-13.md` - Dated launch checklist
- `changes-summary.md` - Connection optimization implementation notes
- `caddy-route-removal-fix.md` - Bug fix documentation
- `auth-migration-summary.md` - Authentication migration summary
- `subdomain-implementation-summary.md` - Subdomain implementation notes
- `server-impact-analysis.md` - Inactivity timeout analysis
- `dual-stream-quick-guide.md` - Quick guide (covered in main architecture doc)
- `docker-to-kubernetes-migration.md` - Migration guide
- `parallel-upload-final-fix.md` - Bug fix documentation
- `fix-redirect-loop-issue.md` - Bug fix documentation
- `getCachedUser-usage.md` - Specific function usage
- `websocket-pooling-implementation.md` - Old websocket pooling system
- `websocket-pool-metrics.md` - Old system metrics
- `websocket-timeout-fix.md` - Bug fix for old system
- `dual-stream-final-implementation.md` - Implementation notes
- `connection-lifetime-strategy.md` - Strategy document

### Phase 2: Manual Cleanup (9 files)
Additional docs removed by maintainer:
- `authentication-architecture.md` - Web auth architecture
- `auth-usage-examples.md` - Auth code examples
- `subdomain-ui-guide.md` - Subdomain UI guide
- `webhook-automation-setup.md` - Webhook automation
- `webhook-deployment.md` - Webhook deployment
- `kubernetes-deployment.md` - K8s deployment guide
- `hetzner-k3s-setup.md` - Hetzner K3s setup
- `metrics-enhancement.md` - Metrics system
- `performance-improvements.md` - Performance notes
- `production-testing-workflow.md` - Testing workflow

**Result**: Reduced from 35 files to 10 files (72% reduction)

## 📝 Contributing to Documentation

When adding new documentation:

1. **Use descriptive filenames** - Clear, lowercase with hyphens
2. **Include date if temporary** - E.g., `migration-2025-12.md` for migration guides
3. **Mark as deprecated** - Add deprecation notice instead of deleting immediately
4. **Update this README** - Add new docs to the appropriate section
5. **Clean up regularly** - Remove outdated implementation notes

## 🆘 Support

For issues or questions:
- Check the relevant documentation first
- Review [production-testing-workflow.md](production-testing-workflow.md) for testing strategies
- Open an issue on GitHub with documentation improvement suggestions
