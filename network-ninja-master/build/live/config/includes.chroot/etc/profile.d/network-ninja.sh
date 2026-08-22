# Network Ninja Master live-session defaults.
export NETWORK_NINJA_USB_ROOT="${NETWORK_NINJA_USB_ROOT:-/media/NETWORK_NINJA_USB}"
export NETWORK_NINJA_DATA_ROOT="${NETWORK_NINJA_DATA_ROOT:-${NETWORK_NINJA_USB_ROOT}/data}"
export NETWORK_NINJA_DASHBOARD_URL="http://127.0.0.1:8000"

if [ -t 1 ]; then
  printf '%s\n' "Network Ninja Master: ${NETWORK_NINJA_DASHBOARD_URL}"
  printf '%s\n' "Persistent data root: ${NETWORK_NINJA_DATA_ROOT}"
fi
