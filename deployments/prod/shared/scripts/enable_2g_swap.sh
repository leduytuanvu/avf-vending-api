#!/usr/bin/env bash
# Add a 2G swapfile if none is active. Resilience only — not a latency knob.
set -euo pipefail
if swapon --show | grep -q .; then
	echo "swap_already_present"
	swapon --show
	exit 0
fi
if [[ -f /swapfile ]]; then
	chmod 600 /swapfile
	mkswap /swapfile
	swapon /swapfile
else
	fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048 status=none
	chmod 600 /swapfile
	mkswap /swapfile
	swapon /swapfile
fi
if ! grep -qE '^/swapfile ' /etc/fstab; then
	echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi
echo "swap_enabled"
swapon --show
free -h | grep -i swap
