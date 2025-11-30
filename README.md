# 🦒 GiraffeCloud

**Secure tunneling service to expose your local services to the internet.**

GiraffeCloud is a self-hostable reverse proxy and tunneling platform. Expose services running on your laptop, home server, or VPS to the internet using custom domains with automatic HTTPS—without revealing your real IP address.

Similar to Cloudflare Tunnel or Ngrok — but fully open-source and self-hosted.

---

## ✨ Features

- 🔒 **Secure Tunneling** - Expose local services safely through encrypted connections
- 🌐 **Custom Domains** - Use your own domains or get a free auto-generated subdomain
- 🔐 **Automatic HTTPS** - Let's Encrypt certificates handled automatically via Caddy
- 🔄 **Auto-Reconnect** - Intelligent retry logic with exponential backoff
- 📊 **Web Dashboard** - Manage tunnels through a modern Next.js interface

---

## 🚀 Quick Start

### Install the CLI

```bash
# macOS/Linux
curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh | bash

# Linux with service (auto-start on boot)
curl -fsSL https://raw.githubusercontent.com/osa911/giraffecloud/main/scripts/install.sh | bash -s -- --service system
```

For Windows and other installation methods, see [docs/installation.md](docs/installation.md)

### Connect Your Service

```bash
# Login to GiraffeCloud
giraffecloud login --token YOUR_API_TOKEN

# Expose your local service
giraffecloud connect

# Check tunnel status
giraffecloud status
```

---

## 🏗️ Architecture

GiraffeCloud uses a hybrid tunnel architecture:

- **gRPC Tunnels** - Unlimited concurrent HTTP requests via HTTP/2 multiplexing
- **TCP Tunnels** - Real-time WebSocket and bidirectional communication
- **Dual-Stream Control** - Separate control channel for instant request cancellation

Key components:
- **Go Backend** - High-performance tunnel server and API
- **Next.js Dashboard** - Modern web interface for tunnel management
- **PostgreSQL** - User and tunnel data persistence
- **Caddy** - Automatic HTTPS and reverse proxy

For technical details, see [docs/hybrid-tunnel-architecture.md](docs/hybrid-tunnel-architecture.md)

---

## 📖 Documentation

- **[Installation Guide](docs/installation.md)** - Detailed installation instructions
- **[Architecture Overview](docs/hybrid-tunnel-architecture.md)** - Technical architecture
- **[Subdomain Feature](docs/subdomain-feature.md)** - Free auto-generated subdomains
- **[VPS Deployment](docs/vps-deployment.md)** - Self-hosting guide

Browse all documentation in [docs/](docs/)

---

## 🛠️ Development

### Prerequisites

- Go 1.21+
- PostgreSQL 15+
- Node.js 22+ (for web dashboard)
- Make

### Local Development

```bash
# Clone repository
git clone https://github.com/osa911/giraffecloud.git
cd giraffecloud

# Set up environment
cp .env.example .env
# Edit .env with your configuration

# Initialize database
make db-init
make db-migrate

# Start development server
make dev (or make dev-hot)

# In another terminal, start web dashboard
cd apps/web
yarn install
yarn dev
```

### Project Structure

```
giraffecloud/
├── cmd/                    # CLI and server entry points
│   ├── giraffecloud/      # CLI client
│   └── server/            # Tunnel server
├── internal/              # Go backend code
│   ├── api/              # API handlers
│   ├── tunnel/           # Tunnel implementation
│   └── db/               # Database layer
├── apps/
│   └── web/              # Next.js dashboard
├── docs/                 # Documentation
└── scripts/             # Installation scripts
```

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

### Development Workflow

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📝 License

This project is licensed under the AGPL-3.0 License - see the [LICENSE](LICENSE) file for details.

---

## 🌍 Links

- **Website**: [giraffecloud.xyz](https://giraffecloud.xyz)
- **GitHub**: [github.com/osa911/giraffecloud](https://github.com/osa911/giraffecloud)
- **Documentation**: [docs/](docs/)

---

## 💡 Use Cases

- **Development** - Share localhost servers with teammates or clients
- **Self-Hosting** - Expose home services without port forwarding
- **Demos** - Show projects without deploying infrastructure
- **IoT** - Connect devices behind NAT/firewalls
- **Testing** - Webhook testing with real domains

---

## 🙏 Acknowledgments

Built with modern technologies:
- [Go](https://golang.org/) - Backend and CLI
- [Next.js](https://nextjs.org/) - Web dashboard
- [PostgreSQL](https://www.postgresql.org/) - Database
- [Caddy](https://caddyserver.com/) - Reverse proxy and HTTPS
- [gRPC](https://grpc.io/) - High-performance RPC framework

---

*The internet should be easy to share — not locked behind limits, logins, or pricing walls.*

*You own the stack. We just help you reach higher.* 🦒
