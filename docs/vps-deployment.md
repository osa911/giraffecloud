# Deploying GiraffeCloud to a VPS

This guide explains how to deploy GiraffeCloud to a VPS using Docker, Docker Compose, and Caddy running on the host.

## Prerequisites

- A VPS running Linux (Ubuntu 20.04+ recommended)
- Domain name pointing to your VPS
- Caddy v2+ installed on the host
- Basic Linux command line knowledge

## Initial Server Setup

1. Connect to your VPS via SSH:

   ```
   ssh user@your-server-ip
   ```

2. Update your system:

   ```
   sudo apt update && sudo apt upgrade -y
   ```

3. Install Docker and Docker Compose if not already installed:

   ```
   # Install Docker
   curl -fsSL https://get.docker.com -o get-docker.sh
   sudo sh get-docker.sh

   # Add your user to the docker group (allows running docker without sudo)
   sudo usermod -aG docker $USER

   # Install Docker Compose
   sudo apt install docker-compose-plugin

   # Log out and back in for group changes to take effect
   exit
   ```

4. Reconnect to your server and verify installations:
   ```
   docker --version
   docker compose version
   ```

## Setting Up Caddy

1. Install Caddy on the host (if not already installed):

   ```
   sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https
   curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
   curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
   sudo apt update
   sudo apt install caddy
   ```

2. Configure Caddy for GiraffeCloud:

   ```
   # Copy the provided Caddyfile.host to Caddy's config directory
   sudo cp configs/caddy/Caddyfile.host /etc/caddy/Caddyfile

   # Edit and update with your domain
   sudo nano /etc/caddy/Caddyfile
   ```

   Replace `example.com` with your actual domain name in the Caddyfile.

3. Restart Caddy to apply changes:
   ```
   sudo systemctl restart caddy
   sudo systemctl status caddy
   ```

## Deploying GiraffeCloud

1. Clone the repository:

   ```
   git clone https://github.com/osa911/giraffecloud.git
   cd giraffecloud
   ```

2. Create Firebase service account file:

   ```
   mkdir -p internal/config/firebase/
   nano internal/config/firebase/service-account.json
   ```

   Paste your Firebase service account JSON content and save (Ctrl+X, then Y, then Enter).

3. Make the deployment script executable:

   ```
   chmod +x scripts/deploy.sh
   chmod +x scripts/extract-db-env.sh
   ```

4. Configure your production environment file:

   ```
   mkdir -p internal/config/env/
   cp internal/config/env/.env.example internal/config/env/.env.production
   nano internal/config/env/.env.production
   ```

   Update the settings with your production values, especially database credentials.

5. Run the deployment script:

   ```
   ./scripts/deploy.sh
   ```

   Select option 1 to build and start the containers.

6. Verify that everything is running correctly:

   ```
   docker ps
   ```

   You should see two containers running: api and postgres.

7. Check that Caddy is correctly proxying to your application:
   ```
   curl -I https://your-domain.com/health
   ```

## Firewall Configuration

Make sure your firewall allows traffic on the necessary ports:

```
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 80/tcp    # HTTP
sudo ufw allow 443/tcp   # HTTPS
```

## Setting Up Automatic Updates

You can use a cron job to automatically update your application:

1. Create an update script:

   ```
   cat > /home/user/update-giraffecloud.sh << EOF
   #!/bin/bash
   cd /path/to/giraffecloud
   git pull
   ./scripts/deploy.sh 2  # Option 2 is for update
   EOF
   ```

2. Make it executable:

   ```
   chmod +x /home/user/update-giraffecloud.sh
   ```

3. Add a cron job to run it weekly:

   ```
   crontab -e
   ```

   Add the following line to run it every Sunday at 3 AM:

   ```
   0 3 * * 0 /home/user/update-giraffecloud.sh >> /home/user/update.log 2>&1
   ```

## Backup Strategy

Database backups are handled by the deployment script. To set up regular backups:

1. Create a backup script:

   ```
   cat > /home/user/backup-giraffecloud.sh << EOF
   #!/bin/bash
   cd /path/to/giraffecloud
   ./scripts/deploy.sh 6  # Option 6 is for database backup
   EOF
   ```

2. Make it executable:

   ```
   chmod +x /home/user/backup-giraffecloud.sh
   ```

3. Add a cron job for daily backups:

   ```
   crontab -e
   ```

   Add the following line to run it daily at 2 AM:

   ```
   0 2 * * * /home/user/backup-giraffecloud.sh >> /home/user/backup.log 2>&1
   ```

## Troubleshooting

### Checking Logs

```
# View all container logs
docker-compose logs

# View logs of a specific container
docker-compose logs api
docker-compose logs postgres

# Check Caddy logs
sudo journalctl -u caddy
```

### Common Issues

1. **Caddy can't obtain SSL certificates**

   - Make sure ports 80 and 443 are open on your VPS firewall
   - Ensure your domain correctly points to your VPS IP address

2. **Database connection issues**

   - Check database credentials in the `.env.production` file
   - Verify that the postgres container is running

3. **API server not starting**
   - Check API logs for detailed error messages
   - Verify that Firebase service account file is correct
