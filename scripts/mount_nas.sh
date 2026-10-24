#!/usr/bin/env bash
# ==============================================================================
# Helper script to mount/unmount Synology NAS SMB shares for R19DEV Studio
# ==============================================================================

set -e

NAS_IP="${NAS_IP:-192.168.1.189}"
NAS_USER="${NAS_USER:-plagad}"
SHARES=("homes" "home")

action="${1:-mount}"

check_ping() {
  echo "📡 Checking NAS reachability at ${NAS_IP}..."
  if ! ping -c 1 -W 1000 "${NAS_IP}" >/dev/null 2>&1; then
    echo "❌ Error: Cannot reach NAS at ${NAS_IP}. Please check network/WiFi connection."
    exit 1
  fi
  echo "✅ NAS is online."
}

mount_share() {
  local share="$1"
  local target_path="/Volumes/${share}"

  if [ -d "${target_path}" ]; then
    echo "ℹ️  '${share}' is already mounted at ${target_path}"
    return 0
  fi

  echo "🔌 Mounting smb://${NAS_USER}@${NAS_IP}/${share}..."
  osascript -e "mount volume \"smb://${NAS_USER}@${NAS_IP}/${share}\"" || {
    echo "⚠️  Failed to mount '${share}'."
    return 1
  }

  # Wait up to 5 seconds for mount to appear
  for i in {1..5}; do
    if [ -d "${target_path}" ]; then
      echo "✅ Successfully mounted: ${target_path}"
      return 0
    fi
    sleep 1
  done

  echo "⚠️  Mounted command executed, but ${target_path} is not yet visible."
}

unmount_share() {
  local share="$1"
  local target_path="/Volumes/${share}"

  if [ ! -d "${target_path}" ]; then
    echo "ℹ️  '${share}' is not mounted."
    return 0
  fi

  echo "⏏️  Unmounting ${target_path}..."
  diskutil unmount "${target_path}" || diskutil unmount force "${target_path}" || {
    echo "❌ Failed to unmount ${target_path}"
    return 1
  }
  echo "✅ Unmounted: ${target_path}"
}

status_share() {
  echo "=== Current NAS Mount Status ==="
  for share in "${SHARES[@]}"; do
    local target_path="/Volumes/${share}"
    if [ -d "${target_path}" ]; then
      echo "  🟢 [MOUNTED] ${target_path}"
      # Check key r19dev directories if available
      if [ "${share}" = "homes" ] && [ -d "/Volumes/homes/${NAS_USER}/BT/organized" ]; then
        echo "     └─ Library: /Volumes/homes/${NAS_USER}/BT/organized (Ready)"
      fi
      if [ "${share}" = "home" ] && [ -d "/Volumes/home/BT/organized" ]; then
        echo "     └─ Library: /Volumes/home/BT/organized (Ready)"
      fi
    else
      echo "  ⚪ [NOT MOUNTED] ${target_path}"
    fi
  done
}

case "${action}" in
  mount)
    check_ping
    echo ""
    for s in "${SHARES[@]}"; do
      mount_share "$s" || true
    done
    echo ""
    status_share
    ;;

  unmount|umount)
    echo ""
    for s in "${SHARES[@]}"; do
      unmount_share "$s" || true
    done
    echo ""
    status_share
    ;;

  status)
    status_share
    ;;

  *)
    echo "Usage: $0 [mount | unmount | status]"
    echo ""
    echo "Environment Variables (Optional):"
    echo "  NAS_IP    (default: 192.168.1.189)"
    echo "  NAS_USER  (default: plagad)"
    exit 1
    ;;
esac
