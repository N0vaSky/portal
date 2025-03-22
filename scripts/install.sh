#!/bin/bash

# Fibratus Portal Installation Script
# This script installs and configures the Fibratus Management Portal on Ubuntu

set -e  # Exit on any error

# Colors for output
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Function to print a status message
print_status() {
    echo -e "${YELLOW}[*]${NC} $1"
}

# Function to print a success message
print_success() {
    echo -e "${GREEN}[+]${NC} $1"
}

# Function to print an error message
print_error() {
    echo -e "${RED}[!]${NC} $1"
}

# Function to check if running as root
check_root() {
    if [ "$(id -u)" -ne 0 ]; then
        print_error "This script must be run as root"
        exit 1
    fi
}

# Function to check Ubuntu version
check_ubuntu() {
    if ! grep -q "Ubuntu" /etc/os-release; then
        print_error "This script is designed for Ubuntu only"
        exit 1
    fi
    
    VERSION=$(grep -oP '(?<=VERSION_ID=")[^"]+' /etc/os-release)
    if [ "${VERSION}" != "20.04" ] && [ "${VERSION}" != "22.04" ]; then
        print_error "This script requires Ubuntu 20.04 or 22.04"
        exit 1
    fi
}

# Function to install dependencies
install_dependencies() {
    print_status "Updating package lists..."
    apt-get update

    print_status "Installing dependencies..."
    apt-get install -y postgresql postgresql-contrib certbot nginx golang-1.19 python3-certbot-nginx

    # Set Go environment variables
    export PATH=$PATH:/usr/lib/go-1.19/bin
}

# Function to set up the PostgreSQL database
setup_database() {
    print_status "Setting up PostgreSQL database..."
    
    # Check if PostgreSQL is running
    if ! systemctl is-active --quiet postgresql; then
        print_status "Starting PostgreSQL..."
        systemctl start postgresql
    fi
    
    # Create database user and database
    sudo -u postgres psql -c "CREATE USER $DB_USER WITH PASSWORD '$DB_PASS';"
    sudo -u postgres psql -c "CREATE DATABASE $DB_NAME WITH OWNER $DB_USER;"
    sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE $DB_NAME TO $DB_USER;"
    
    print_success "Database setup completed"
}

# Function to set up the Fibratus directories
setup_directories() {
    print_status "Setting up Fibratus directories..."
    
    # Create necessary directories
    mkdir -p /etc/fibratus/migrations
    mkdir -p /etc/fibratus/rules
    mkdir -p /var/lib/fibratus
    mkdir -p /usr/share/fibratus/web
    
    # Set proper permissions
    chown -R fibratus:fibratus /etc/fibratus
    chown -R fibratus:fibratus /var/lib/fibratus
    chown -R fibratus:fibratus /usr/share/fibratus
    
    print_success "Directories created"
}

# Function to set up the Fibratus system user
setup_user() {
    print_status "Setting up Fibratus system user..."
    
    # Create system user if it doesn't exist
    if ! id -u fibratus &>/dev/null; then
        useradd -r -s /bin/false -m -d /var/lib/fibratus fibratus
    fi
    
    print_success "System user created"
}

# Function to generate TLS certificates
setup_tls() {
    print_status "Setting up TLS..."
    
    if [ "$USE_LETSENCRYPT" = "true" ]; then
        # Use Let's Encrypt
        print_status "Obtaining Let's Encrypt certificate for $DOMAIN..."
        certbot --nginx -d $DOMAIN --non-interactive --agree-tos -m $EMAIL
        
        # Link certificates to Fibratus directory
        ln -sf /etc/letsencrypt/live/$DOMAIN/fullchain.pem /etc/fibratus/cert.pem
        ln -sf /etc/letsencrypt/live/$DOMAIN/privkey.pem /etc/fibratus/key.pem
    else
        # Generate self-signed certificate
        print_status "Generating self-signed certificate..."
        openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
            -keyout /etc/fibratus/key.pem \
            -out /etc/fibratus/cert.pem \
            -subj "/CN=$DOMAIN/O=Fibratus/C=US"
    fi
    
    # Set proper permissions
    chmod 600 /etc/fibratus/key.pem
    chmod 644 /etc/fibratus/cert.pem
    chown fibratus:fibratus /etc/fibratus/key.pem
    chown fibratus:fibratus /etc/fibratus/cert.pem
    
    print_success "TLS setup completed"
}

# Function to configure Fibratus Portal
configure_fibratus() {
    print_status "Configuring Fibratus Portal..."
    
    # Create server configuration
    cat > /etc/fibratus/server.yaml << EOF
log_level: info
server:
  host: $SERVER_HOST
  port: $SERVER_PORT
  tls:
    enabled: true
    cert_file: /etc/fibratus/cert.pem
    key_file: /etc/fibratus/key.pem
database:
  host: localhost
  port: 5432
  username: $DB_USER
  password: $DB_PASS
  database: $DB_NAME
  ssl_mode: disable
auth:
  jwt_secret: $JWT_SECRET
  mfa_enabled: $MFA_ENABLED
  session_key: $SESSION_KEY
  session_name: fibratus_session
  cookie_secure: true
fibratus:
  heartbeat_interval: 60
  heartbeat_timeout: 180
  alerts_json_path: /var/lib/fibratus/alerts.json
  default_rules_dir_path: /etc/fibratus/rules
EOF

    # Set proper permissions
    chmod 600 /etc/fibratus/server.yaml
    chown fibratus:fibratus /etc/fibratus/server.yaml
    
    print_success "Fibratus configuration created"
}

# Function to set up systemd service
setup_service() {
    print_status "Setting up systemd service..."
    
    # Create systemd service file
    cat > /etc/systemd/system/fibratus-portal.service << EOF
[Unit]
Description=Fibratus Management Portal
After=network.target postgresql.service

[Service]
User=fibratus
Group=fibratus
ExecStart=/usr/bin/fibratus-server -config /etc/fibratus/server.yaml
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

    # Reload systemd
    systemctl daemon-reload
    
    print_success "Systemd service created"
}

# Function to set up Nginx
setup_nginx() {
    print_status "Setting up Nginx as reverse proxy..."
    
    # Create Nginx configuration
    cat > /etc/nginx/sites-available/fibratus << EOF
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
    
    location / {
        proxy_pass http://127.0.0.1:$SERVER_PORT;
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
    ln -sf /etc/nginx/sites-available/fibratus /etc/nginx/sites-enabled/
    
    # Test and reload Nginx
    nginx -t
    systemctl reload nginx
    
    print_success "Nginx configured as reverse proxy"
}

# Function to create initial admin user
create_admin_user() {
    print_status "Creating initial admin user..."
    
    # Wait for the service to start
    sleep 3
    
    # Make API request to create admin user
    curl -X POST -H "Content-Type: application/json" \
        -d "{\"username\":\"$ADMIN_USER\",\"password\":\"$ADMIN_PASS\",\"role\":\"admin\"}" \
        http://127.0.0.1:$SERVER_PORT/api/v1/users/init
    
    print_success "Admin user created"
}

# Function to display a summary of the installation
display_summary() {
    print_status "Installation completed successfully!"
    echo ""
    echo "Summary:"
    echo "========"
    echo "Fibratus Portal URL: https://$DOMAIN"
    echo "Admin username: $ADMIN_USER"
    echo "Database: $DB_NAME"
    echo ""
    echo "Listening ports:"
    echo "- Web interface: 443 (HTTPS via Nginx)"
    echo "- API service: $SERVER_PORT (internal, proxied via Nginx)"
    echo "- PostgreSQL: 5432 (internal)"
    echo ""
    echo "Configuration files:"
    echo "- Main config: /etc/fibratus/server.yaml"
    echo "- Rules directory: /etc/fibratus/rules"
    echo "- Alerts JSON: /var/lib/fibratus/alerts.json"
    echo ""
    echo "If you need to reset the admin password, use:"
    echo "sudo -u fibratus fibratus-server -reset-password -username $ADMIN_USER -config /etc/fibratus/server.yaml"
    echo ""
    echo "Make sure to secure your server with a firewall!"
}

# Main installation process
main() {
    print_status "Starting Fibratus Portal installation..."
    
    # Check prerequisites
    check_root
    check_ubuntu
    
    # Collect installation parameters
    read -p "Enter admin username [admin]: " ADMIN_USER
    ADMIN_USER=${ADMIN_USER:-admin}
    
    read -sp "Enter admin password: " ADMIN_PASS
    echo ""
    read -sp "Confirm admin password: " ADMIN_PASS_CONFIRM
    echo ""
    
    if [ "$ADMIN_PASS" != "$ADMIN_PASS_CONFIRM" ]; then
        print_error "Passwords do not match"
        exit 1
    fi
    
    read -p "Enable MFA (Multi-Factor Authentication)? (y/n) [y]: " ENABLE_MFA
    ENABLE_MFA=${ENABLE_MFA:-y}
    if [ "$ENABLE_MFA" = "y" ] || [ "$ENABLE_MFA" = "Y" ]; then
        MFA_ENABLED=true
    else
        MFA_ENABLED=false
    fi
    
    read -p "Enter domain name [fibratus.local]: " DOMAIN
    DOMAIN=${DOMAIN:-fibratus.local}
    
    read -p "Use Let's Encrypt for TLS certificate? (y/n) [n]: " USE_LETSENCRYPT_CHOICE
    USE_LETSENCRYPT_CHOICE=${USE_LETSENCRYPT_CHOICE:-n}
    if [ "$USE_LETSENCRYPT_CHOICE" = "y" ] || [ "$USE_LETSENCRYPT_CHOICE" = "Y" ]; then
        USE_LETSENCRYPT=true
        read -p "Enter email address for Let's Encrypt: " EMAIL
    else
        USE_LETSENCRYPT=false
    fi
    
    read -p "Enter database name [fibratus]: " DB_NAME
    DB_NAME=${DB_NAME:-fibratus}
    
    read -p "Enter database username [fibratus]: " DB_USER
    DB_USER=${DB_USER:-fibratus}
    
    read -sp "Enter database password: " DB_PASS
    echo ""
    read -sp "Confirm database password: " DB_PASS_CONFIRM
    echo ""
    
    if [ "$DB_PASS" != "$DB_PASS_CONFIRM" ]; then
        print_error "Database passwords do not match"
        exit 1
    fi
    
    read -p "Enter server bind address [0.0.0.0]: " SERVER_HOST
    SERVER_HOST=${SERVER_HOST:-0.0.0.0}
    
    read -p "Enter server port [8080]: " SERVER_PORT
    SERVER_PORT=${SERVER_PORT:-8080}
    
    # Generate random secrets
    JWT_SECRET=$(openssl rand -hex 32)
    SESSION_KEY=$(openssl rand -hex 32)
    
    # Perform installation steps
    install_dependencies
    setup_user
    setup_directories
    setup_database
    setup_tls
    configure_fibratus
    setup_service
    setup_nginx
    
    # Start the service
    print_status "Starting Fibratus Portal service..."
    systemctl enable fibratus-portal
    systemctl start fibratus-portal
    
    # Create initial admin user
    create_admin_user
    
    # Display summary
    display_summary
}

# Start installation
main