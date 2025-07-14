#!/bin/sh
# Script that runs on boot for a workstations VM

# Add all sshd session users to the docker group
[ $(grep "docker" /etc/security/group.conf) ] || echo 'sshd;*;*;Al0000-2400;docker' >> /etc/security/group.conf