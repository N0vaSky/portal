# Fibratus Portal Deployment Guide

This guide provides step-by-step instructions for deploying the Fibratus Portal on an Ubuntu server.

## Prerequisites

- Ubuntu Server 20.04 LTS or 22.04 LTS
- Sudo/root access
- Internet connection

## Deployment Methods

There are two methods for deploying the Fibratus Portal:

1. **Building from source** (recommended for development/customization)
2. **Using the Debian package** (recommended for production)

## Method 1: Building from Source

### Step 1: Install Dependencies

```bash
# Update package lists
sudo apt update

# Install required packages
sudo apt install -y golang-1.19 postgresql postgresql-contrib nginx certbot python3-certbot-nginx git build-essential
```

### Step 2: Clone the Project

Since this is a local project without a GitHub repository, you'll need to copy the project files to your server.

```bash
# Create directory for the project
mkdir -p ~/fibratus-portal
cd ~/fibratus-portal

# [Alternative: If using git, you would do the following]
# git clone <your-repo-url> .
```

Transfer your local project files to this directory using SCP, SFTP, or another file transfer method:

```bash
# From your local machine (not on the server):
scp -r /path/to/fibratus-portal/* user@your-server-ip:~/fibratus-portal/
```

### Step 3: Build the Project

```bash
cd ~/fibratus-portal

# Set up Go environment
export PATH=$PATH:/usr/lib/go-1.19/bin
export GOROOT=/usr/lib/go-1.19

# Build the binary
make build
```

### Step 4: Set Up the Database

```bash
# Create PostgreSQL user and database
sudo -u postgres psql -c "CREATE USER fibratus WITH PASSWORD 'your-secure-password';"
sudo -u postgres psql -c "CREATE DATABASE fibratus WITH OWNER fibratus;"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE fibratus TO fibratus;"
```

### Step 5: Create System User

```bash
# Create a system user for the service
sudo useradd -r -s /bin/false -m -d /var/lib/fibratus fibratus
```

### Step 6: Set Up Directories

```bash
# Create necessary directories
sudo mkdir -p /etc/fibratus/migrations
sudo mkdir -p /etc/fibratus/rules
sudo mkdir -p /var/lib/fibratus
sudo mkdir -p /usr/share/fibratus/web

# Copy files to appropriate locations
sudo cp -r ~/fibratus-portal/migrations/* /etc/fibratus/migrations/
sudo cp -r ~/fibratus-portal/web/* /usr/share/fibratus/web/
sudo cp ~/fibratus-portal/fibratus-server /usr/bin/

# Set proper permissions
sudo chown -R fibratus:fibratus /etc/fibratus
sudo chown -R fibratus:fibratus /var/lib/fibratus
sudo chown -R fibratus:fibratus /usr/share/fibratus
sudo chmod 755 /usr/bin/fibratus-server
```

### Step 7: Configure the Portal

Create the configuration file:

```bash
sudo bash -c 'cat > /etc/fibratus/server.yaml' << EOF
log_level: info
server:
  host: 127.0.0.1
  port: 8080
  tls:
    enabled: false  # We'll use Nginx as a TLS termination proxy
database:
  host: localhost
  port: 5432
  username: fibratus
  password: your-secure-password
  database: fibratus
  ssl_mode: disable
auth:
  jwt_secret: $(openssl rand -hex 32)
  mfa_enabled: true
  session_key: $(openssl rand -hex 32)
  session_name: fibratus_session
  cookie_secure: true
fibratus:
  heartbeat_interval: 60
  heartbeat_timeout: 180
  alerts_json_path: /var/lib/fibratus/alerts.json
  default_rules_dir_path: /etc/fibratus/rules
EOF

sudo chmod 600 /etc/fibratus/server.yaml
sudo chown fibratus:fibratus /etc/fibratus/server.yaml
```

### Step 8: Set Up Systemd Service

```bash
sudo bash -c 'cat > /etc/systemd/system/fibratus-portal.service' << EOF
[Unit]
Description=Fibratus Management Portal
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=fibratus
Group=fibratus
ExecStart=/usr/bin/fibratus-server -config /etc/fibratus/server.yaml
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

# Security hardening
ProtectSystem=full
PrivateTmp=true
NoNewPrivileges=true
PrivateDevices=true
ProtectHome=true

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable fibratus-portal.service
```

### Step 9: Configure Nginx

```bash
# Set the domain name
DOMAIN="your-domain.com"  # Change this to your domain or IP address

# Create Nginx configuration
sudo bash -c "cat > /etc/nginx/sites-available/fibratus-portal" << EOF
server {
    listen 80;
    server_name $DOMAIN;
    
    location / {
        return 301 https://\$host\$request_uri;
    }
}

server {
    listen 443 ssl;
    server_name $DOMAIN;
    
    ssl_certificate /etc/fibratus/cert.pem;
    ssl_certificate_key /etc/fibratus/key.pem;
    
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_prefer_server_ciphers on;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES256-GCM-SHA384;
    ssl_session_timeout 1d;
    ssl_session_cache shared:SSL:10m;
    ssl_session_tickets off;
    
    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options nosniff;
    add_header X-Frame-Options SAMEORIGIN;
    add_header X-XSS-Protection "1; mode=block";
    
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_cache_bypass \$http_upgrade;
    }
}
EOF

# Enable the site
sudo ln -sf /etc/nginx/sites-available/fibratus-portal /etc/nginx/sites-enabled/

# Remove default site if it exists
sudo rm -f /etc/nginx/sites-enabled/default
```

### Step 10: Set Up SSL Certificate

#### Option A: Using Let's Encrypt (recommended for production with a domain)

```bash
sudo certbot --nginx -d $DOMAIN

# Update Nginx configuration to use the new certificates
sudo sed -i "s|ssl_certificate /etc/fibratus/cert.pem;|ssl_certificate /etc/letsencrypt/live/$DOMAIN/fullchain.pem;|g" /etc/nginx/sites-available/fibratus-portal
sudo sed -i "s|ssl_certificate_key /etc/fibratus/key.pem;|ssl_certificate_key /etc/letsencrypt/live/$DOMAIN/privkey.pem;|g" /etc/nginx/sites-available/fibratus-portal
```

#### Option B: Using Self-Signed Certificate (for testing)

```bash
sudo openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout /etc/fibratus/key.pem \
  -out /etc/fibratus/cert.pem \
  -subj "/CN=$DOMAIN/O=Fibratus/C=US"

sudo chmod 600 /etc/fibratus/key.pem
sudo chmod 644 /etc/fibratus/cert.pem
sudo chown fibratus:fibratus /etc/fibratus/key.pem /etc/fibratus/cert.pem
```

### Step 11: Start Services

```bash
# Test and reload Nginx
sudo nginx -t
sudo systemctl reload nginx

# Start the Fibratus Portal
sudo systemctl start fibratus-portal
```

### Step 12: Create Initial Admin User

After the service is running, you need to create an initial admin user. Here's a simple script to do that:

```bash
cat > ~/create_admin.sh << 'EOF'
#!/bin/bash
read -p "Enter admin username [admin]: " USERNAME
USERNAME=${USERNAME:-admin}
read -sp "Enter admin password: " PASSWORD
echo
read -sp "Confirm admin password: " PASSWORD_CONFIRM
echo

if [ "$PASSWORD" != "$PASSWORD_CONFIRM" ]; then
  echo "Passwords do not match!"
  exit 1
fi

curl -X POST -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\",\"role\":\"admin\"}" \
  http://localhost:8080/api/v1/users/init

echo
echo "Admin user created. You can now log in at https://$DOMAIN"
EOF

chmod +x ~/create_admin.sh
~/create_admin.sh
```

## Method 2: Using the Debian Package

### Step 1: Build the Debian Package (on Development Machine)

```bash
# On your development machine
cd ~/fibratus-portal

# Install packaging tools
sudo apt install -y build-essential debhelper dh-make dh-golang

# Build the package
make package

# This will create a .deb file in the current directory
```

### Step 2: Transfer the Debian Package to the Server

```bash
# From your development machine
scp fibratus-portal_*.deb user@your-server-ip:~/
```

### Step 3: Install the Package on the Server

```bash
# On the server
sudo apt update
sudo apt install -y postgresql nginx certbot python3-certbot-nginx

# Install the Fibratus Portal package
sudo dpkg -i ~/fibratus-portal_*.deb
sudo apt -f install -y
```

The installation script should run automatically and guide you through the setup process, including:

- Database setup
- User creation
- SSL certificate generation
- Service configuration

### Step 4: Access the Portal

Once the installation is complete, you can access the portal at:

```
https://your-server-ip/
```

Or if you configured a domain:

```
https://your-domain.com/
```

## Verifying the Installation

Check the service status:

```bash
sudo systemctl status fibratus-portal
```

Check the logs:

```bash
sudo journalctl -u fibratus-portal
```

## Troubleshooting

### Database Connection Issues

If the portal can't connect to the database:

```bash
# Check PostgreSQL status
sudo systemctl status postgresql

# Verify PostgreSQL configuration
sudo nano /etc/postgresql/*/main/pg_hba.conf

# Restart PostgreSQL
sudo systemctl restart postgresql
```

### Web Interface Not Accessible

If you can't access the web interface:

```bash
# Check Nginx status
sudo systemctl status nginx

# Verify Nginx configuration
sudo nginx -t

# Check firewall settings
sudo ufw status

# Allow HTTP and HTTPS if using a firewall
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

### Service Not Starting

If the Fibratus Portal service won't start:

```bash
# Check detailed logs
sudo journalctl -u fibratus-portal -n 100

# Verify file permissions
sudo chown -R fibratus:fibratus /etc/fibratus /var/lib/fibratus /usr/share/fibratus

# Check configuration file
sudo nano /etc/fibratus/server.yaml
```

## Ports Used

The Fibratus Portal uses the following ports:

- **80**: HTTP (redirects to HTTPS)
- **443**: HTTPS (Nginx)
- **8080**: Internal service (not exposed to the internet)
- **5432**: PostgreSQL (not exposed to the internet)

Make sure these ports are allowed in your firewall if applicable.

## Security Considerations

1. Change default passwords for the admin user and database
2. Use Let's Encrypt for production environments
3. Configure a firewall to restrict access to necessary ports only
4. Regularly update your system and the Fibratus Portal
5. Consider implementing additional security measures like fail2ban and IP restrictions

## Backup and Recovery

### Database Backup

```bash
# Create a backup of the database
sudo -u postgres pg_dump fibratus > fibratus_backup.sql

# Restore from backup
sudo -u postgres psql fibratus < fibratus_backup.sql
```

### Configuration Backup

```bash
# Back up configuration files
sudo cp -r /etc/fibratus /etc/fibratus.bak
```

## Upgrading

When a new version is released:

1. Stop the service: `sudo systemctl stop fibratus-portal`
2. Back up the database and configuration
3. Install the new package or deploy the new code
4. Run database migrations if needed
5. Start the service: `sudo systemctl start fibratus-portal`

## Uninstallation

If you need to remove the Fibratus Portal:

```bash
# If installed from package
sudo apt remove --purge fibratus-portal

# If installed from source
sudo systemctl stop fibratus-portal
sudo systemctl disable fibratus-portal
sudo rm /etc/systemd/system/fibratus-portal.service
sudo rm -rf /etc/fibratus /var/lib/fibratus /usr/share/fibratus
sudo rm /usr/bin/fibratus-server
sudo userdel -r fibratus
sudo -u postgres psql -c "DROP DATABASE fibratus;"
sudo -u postgres psql -c "DROP USER fibratus;"
```

## Support

For support, please refer to the documentation or contact your system administrator.

---

This deployment guide was last updated on March 22, 2025.