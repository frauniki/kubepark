FROM ubuntu:22.04

# Install OpenSSH Server and other utilities
RUN apt-get update && \
    apt-get install -y openssh-server curl iputils-ping net-tools vim && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Configure SSH
RUN mkdir -p /var/run/sshd && \
    mkdir -p /root/.ssh && \
    chmod 700 /root/.ssh && \
    echo "PermitRootLogin prohibit-password" >> /etc/ssh/sshd_config && \
    echo "AuthorizedKeysFile /etc/ssh/authorized_keys" >> /etc/ssh/sshd_config && \
    sed -i 's/#PasswordAuthentication yes/PasswordAuthentication no/' /etc/ssh/sshd_config

# Create a non-root user
RUN useradd -m -s /bin/bash sandbox && \
    mkdir -p /home/sandbox/.ssh && \
    chmod 700 /home/sandbox/.ssh && \
    chown -R sandbox:sandbox /home/sandbox

# Copy entrypoint script
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Expose SSH port
EXPOSE 22

# Set entrypoint
ENTRYPOINT ["/entrypoint.sh"]
