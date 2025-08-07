#!/bin/bash

set -e

# Get SSH username from environment variable, default to 'sandbox'
SSH_USER=${SSH_USERNAME:-sandbox}

echo "Setting up SSH configuration for user: $SSH_USER"

# Create user if it doesn't exist
if ! id "$SSH_USER" &>/dev/null; then
  echo "Creating user: $SSH_USER"
  useradd -m -s /bin/bash "$SSH_USER"
  echo "User $SSH_USER created successfully"
else
  echo "User $SSH_USER already exists"
fi

# Ensure home directory exists and has correct permissions
USER_HOME="/home/$SSH_USER"
if [ ! -d "$USER_HOME" ]; then
  echo "Creating home directory for $SSH_USER"
  mkdir -p "$USER_HOME"
  chown "$SSH_USER:$SSH_USER" "$USER_HOME"
fi

# Setup SSH directory and permissions for the user
echo "Setting up SSH directory for $SSH_USER"
mkdir -p "$USER_HOME/.ssh"
chmod 700 "$USER_HOME/.ssh"
chown "$SSH_USER:$SSH_USER" "$USER_HOME/.ssh"

# Copy authorized_keys from ConfigMap mount point
AUTHORIZED_KEYS_FILE="$USER_HOME/.ssh/authorized_keys"
if [ -f /tmp/ssh-config/authorized_keys ]; then
  echo "Copying authorized_keys from ConfigMap..."
  cp /tmp/ssh-config/authorized_keys "$AUTHORIZED_KEYS_FILE"
elif [ -f /etc/ssh/authorized_keys ]; then
  # Fallback for backward compatibility
  echo "Copying authorized_keys from /etc/ssh (fallback)..."
  cp /etc/ssh/authorized_keys "$AUTHORIZED_KEYS_FILE"
else
  echo "Warning: No authorized_keys file found"
fi

# Set proper permissions for authorized_keys
if [ -f "$AUTHORIZED_KEYS_FILE" ]; then
  echo "Setting permissions for SSH files..."
  chmod 600 "$AUTHORIZED_KEYS_FILE"
  chown "$SSH_USER:$SSH_USER" "$AUTHORIZED_KEYS_FILE"
  echo "SSH key setup completed for user $SSH_USER"
else
  echo "Warning: authorized_keys file not found, SSH key authentication may not work"
fi

# Generate SSH host keys if they don't exist
if [ ! -f /etc/ssh/ssh_host_rsa_key ]; then
  echo "Generating SSH host keys..."
  ssh-keygen -A
fi

# Create custom sshd_config to use correct authorized_keys path and user
echo "Creating custom SSHD configuration for user $SSH_USER..."
cat > /tmp/sshd_config <<EOF
# Custom SSHD configuration for sandbox user: $SSH_USER
Port 22
Protocol 2
HostKey /etc/ssh/ssh_host_rsa_key
HostKey /etc/ssh/ssh_host_ecdsa_key
HostKey /etc/ssh/ssh_host_ed25519_key

# Authentication
AuthorizedKeysFile $USER_HOME/.ssh/authorized_keys
PubkeyAuthentication yes
PasswordAuthentication no
ChallengeResponseAuthentication no
UsePAM yes

# Security settings
PermitRootLogin no
AllowUsers $SSH_USER
StrictModes yes
MaxAuthTries 3
MaxSessions 10

# Logging
SyslogFacility AUTH
LogLevel INFO

# Other settings
X11Forwarding yes
PrintMotd no
AcceptEnv LANG LC_*
Subsystem sftp /usr/lib/openssh/sftp-server
EOF

echo "SSHD configuration created for user: $SSH_USER"
echo "Starting SSH daemon..."
exec /usr/sbin/sshd -D -f /tmp/sshd_config
