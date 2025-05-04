set -e

if [ -f /etc/ssh/authorized_keys ]; then
  cp /etc/ssh/authorized_keys /home/sandbox/.ssh/authorized_keys
  chmod 600 /home/sandbox/.ssh/authorized_keys
  chown sandbox:sandbox /home/sandbox/.ssh/authorized_keys
fi

if [ ! -f /etc/ssh/ssh_host_rsa_key ]; then
  ssh-keygen -A
fi

exec /usr/sbin/sshd -D
