#!/bin/bash

set -e

# Setup SSH directory and permissions for sandbox user
echo "Setting up SSH configuration..."
mkdir -p /home/sandbox/.ssh

# Copy authorized_keys from ConfigMap mount point
if [ -f /tmp/ssh-config/authorized_keys ]; then
  echo "Copying authorized_keys from ConfigMap..."
  cp /tmp/ssh-config/authorized_keys /home/sandbox/.ssh/authorized_keys
elif [ -f /etc/ssh/authorized_keys ]; then
  # Fallback for backward compatibility
  echo "Copying authorized_keys from /etc/ssh (fallback)..."
  cp /etc/ssh/authorized_keys /home/sandbox/.ssh/authorized_keys
fi

# Set proper permissions
if [ -f /home/sandbox/.ssh/authorized_keys ]; then
  echo "Setting permissions for SSH files..."
  chmod 755 /home/sandbox
  chmod 700 /home/sandbox/.ssh
  chmod 600 /home/sandbox/.ssh/authorized_keys
  chown -R sandbox:sandbox /home/sandbox
fi

# Generate SSH host keys if they don't exist
if [ ! -f /etc/ssh/ssh_host_rsa_key ]; then
  echo "Generating SSH host keys..."
  ssh-keygen -A
fi

# Create custom sshd_config to use correct authorized_keys path
echo "Creating custom SSHD configuration..."
cat > /tmp/sshd_config <<EOF
# Custom SSHD configuration for sandbox
AuthorizedKeysFile /home/sandbox/.ssh/authorized_keys
PubkeyAuthentication yes
PasswordAuthentication no
PermitRootLogin no
AllowUsers sandbox
StrictModes yes
EOF

# Append the rest of the default config
cat /etc/ssh/sshd_config >> /tmp/sshd_config

echo "Starting SSH daemon..."
exec /usr/sbin/sshd -D -f /tmp/sshd_config
