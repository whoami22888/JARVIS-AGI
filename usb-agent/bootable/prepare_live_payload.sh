#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
INCLUDE_DIR="${SCRIPT_DIR}/config/includes.chroot/opt/usb-agent"
AUTOSTART_DIR="${SCRIPT_DIR}/config/includes.chroot/etc/skel/.config/autostart"

rm -rf "${INCLUDE_DIR}"
mkdir -p "${INCLUDE_DIR}" "${AUTOSTART_DIR}"

# Debian Bookworm stores isolinux and vesamenu modules in paths that differ
# from the absolute links bundled with this older live-build template.
BOOT_TEMPLATE_DIR="${SCRIPT_DIR}/config/bootloaders/isolinux"
rm -rf "${BOOT_TEMPLATE_DIR}"
mkdir -p "$(dirname "${BOOT_TEMPLATE_DIR}")"
cp -a /usr/share/live/build/bootloaders/isolinux "${BOOT_TEMPLATE_DIR}"
ln -sfn /usr/lib/ISOLINUX/isolinux.bin "${BOOT_TEMPLATE_DIR}/isolinux.bin"
ln -sfn /usr/lib/syslinux/modules/bios/vesamenu.c32 "${BOOT_TEMPLATE_DIR}/vesamenu.c32"
# Older live-build releases always post-process a bootlogo cpio archive.
# Supply a minimal valid archive for the neutral built-in theme.
mkdir -p "${BOOT_TEMPLATE_DIR}/.bootlogo"
printf 'USB Agent Live\n' > "${BOOT_TEMPLATE_DIR}/.bootlogo/README"
( cd "${BOOT_TEMPLATE_DIR}/.bootlogo" && find . -print | cpio --quiet -o ) > "${BOOT_TEMPLATE_DIR}/bootlogo"
rm -rf "${BOOT_TEMPLATE_DIR}/.bootlogo"

for item in backend frontend models config scripts docs README.md; do
  cp -a "${ROOT}/${item}" "${INCLUDE_DIR}/"
done
mkdir -p "${INCLUDE_DIR}/data"
cp "${ROOT}/config/.env.example" "${INCLUDE_DIR}/.env"
sed -i 's#USB_AGENT_HOME=.*#USB_AGENT_HOME=/opt/usb-agent#; s#USB_AGENT_DATA=.*#USB_AGENT_DATA=/home/user/.local/share/usb-agent#; s#USB_AGENT_CONFIG=.*#USB_AGENT_CONFIG=/opt/usb-agent/config#' "${INCLUDE_DIR}/.env"

cat > "${AUTOSTART_DIR}/usb-agent.desktop" <<'EOF'
[Desktop Entry]
Type=Application
Name=USB Agent Command Center
Comment=Start the local USB Agent gateway and dashboard
Exec=/opt/usb-agent/scripts/launch.sh
Terminal=false
X-GNOME-Autostart-enabled=true
EOF

chmod +x "${INCLUDE_DIR}/scripts/launch.sh" "${INCLUDE_DIR}/scripts/pull_models.sh" "${INCLUDE_DIR}/scripts/benchmark.sh"
printf 'Prepared live payload at %s\n' "${INCLUDE_DIR}"
