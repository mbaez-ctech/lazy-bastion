# lazy-bastion (lzb) – instalador Windows (PowerShell)
# Uso: iwr https://OWNER.github.io/lazy-bastion/install.ps1 | iex
$ErrorActionPreference = "Stop"

# ─── configuración ────────────────────────────────────────────────────────────
$Repo       = "OWNER/lazy-bastion"           # <-- cambia esto por tu org/repo
$Binary     = "lzb"
$InstallDir = "$env:LOCALAPPDATA\Programs\lzb"
# ──────────────────────────────────────────────────────────────────────────────

function Write-Step  { Write-Host "▶ $args" -ForegroundColor Green }
function Write-Warn  { Write-Host "⚠  $args" -ForegroundColor Yellow }
function Write-Fail  { Write-Host "✗  $args" -ForegroundColor Red; exit 1 }

# Verificar arquitectura (solo x64 soportado)
if ($env:PROCESSOR_ARCHITECTURE -ne "AMD64" -and $env:PROCESSOR_ARCHITEW6432 -ne "AMD64") {
    Write-Fail "Solo se soporta Windows x64 (AMD64)."
}

# Resolver versión
$Version = $env:LZB_VERSION
if (-not $Version) {
    Write-Step "Consultando última versión..."
    try {
        $release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
        $Version = $release.tag_name.TrimStart("v")
    } catch {
        Write-Fail "No se pudo obtener la versión. Verifica que el repo '$Repo' existe y tiene releases."
    }
}

$Filename  = "${Binary}-windows-amd64.exe"
$Url       = "https://github.com/$Repo/releases/download/v$Version/$Filename"
$TmpPath   = "$env:TEMP\${Binary}.exe"
$FinalPath = "$InstallDir\${Binary}.exe"

Write-Step "Instalando lzb v$Version (windows/amd64)..."
Write-Step "Descargando desde: $Url"

# Crear directorio de instalación
New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null

# Descargar
try {
    Invoke-WebRequest -Uri $Url -OutFile $TmpPath -UseBasicParsing
} catch {
    Write-Fail "Descarga fallida. Verifica que la version v$Version existe en GitHub Releases."
}

Move-Item -Force $TmpPath $FinalPath

# Agregar al PATH de usuario si no está
$CurrentPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($CurrentPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$CurrentPath;$InstallDir", "User")
    Write-Warn "Agregado $InstallDir al PATH. Reinicia la terminal para que tome efecto."
}

Write-Host ""
Write-Host "✓ lzb v$Version instalado en $FinalPath" -ForegroundColor Green
Write-Host ""
Write-Host "  Próximo paso: coloca tu config.yml y corre 'lzb'"
Write-Host "  Docs: https://OWNER.github.io/lazy-bastion"
Write-Host ""
