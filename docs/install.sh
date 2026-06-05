#!/bin/bash
set -e

# ─── configuración ────────────────────────────────────────────────────────────
REPO="mbaez-ctech/lazy-bastion"
BINARY="lzb"
VERSION="${LZB_VERSION:-latest}"
INSTALL_DIR="${LZB_INSTALL_DIR:-/usr/local/bin}"
# ──────────────────────────────────────────────────────────────────────────────

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
info()  { echo -e "${GREEN}▶${NC} $*"; }
warn()  { echo -e "${YELLOW}⚠${NC}  $*"; }
error() { echo -e "${RED}✗${NC}  $*" >&2; exit 1; }

# Detectar OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  darwin) OS="darwin" ;;
  linux)  OS="linux"  ;;
  *)      error "Sistema operativo no soportado: $OS" ;;
esac

# Detectar arquitectura
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)       ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)             error "Arquitectura no soportada: $ARCH" ;;
esac

# Resolver versión latest
if [ "$VERSION" = "latest" ]; then
  info "Consultando última versión..."
  VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')
  [ -z "$VERSION" ] && error "No se pudo obtener la versión. Verifica que el repo '${REPO}' existe y tiene releases."
fi

FILENAME="${BINARY}-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${FILENAME}"
TMP="/tmp/${BINARY}"

info "Instalando lzb v${VERSION} (${OS}/${ARCH})..."
info "Descargando desde: ${URL}"

curl -fsSL --progress-bar "$URL" -o "$TMP" || error "Descarga fallida. Verifica que la versión v${VERSION} existe en GitHub Releases."
chmod +x "$TMP"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "${INSTALL_DIR}/${BINARY}"
else
  warn "Se requieren permisos de administrador para instalar en ${INSTALL_DIR}"
  sudo mv "$TMP" "${INSTALL_DIR}/${BINARY}"
fi

echo ""
echo -e "${GREEN}✓ lzb v${VERSION} instalado en ${INSTALL_DIR}/${BINARY}${NC}"
echo ""
echo "  Próximo paso: coloca tu config.yml y corre 'lzb'"
echo "  Docs: https://mbaez-ctech.github.io/lazy-bastion"
echo ""
