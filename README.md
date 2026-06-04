# lazy-bastion (`lzb`)

TUI en Go para gestionar túneles AWS SSM y conexiones SSH a servidores de producción de WorldBinary.

## Requisitos

| Herramienta | Versión mínima |
|---|---|
| Go | 1.22 |
| AWS CLI v2 | 2.x |
| Session Manager Plugin | última |
| `lsof` | cualquier |
| `ssh-keygen` | cualquier |

El usuario AWS debe pertenecer al grupo SSO `Admin_Tech` o `BastionAccess`.

## Compilar

```bash
go build -o lzb .
```

El binario `lzb` queda en el directorio actual. Cópialo donde sea conveniente:

```bash
cp lzb /usr/local/bin/lzb
```

## Configuración

El archivo `config.yml` se busca en este orden:

1. Variable de entorno `LZB_CONFIG`
2. Junto al binario
3. Directorio actual
4. `~/.config/lazy-bastion/config.yml`

### Estructura de `config.yml`

```yaml
aws:
  region: us-west-1          # región de los recursos
  account_id: "123456789012" # cuenta AWS (para auto-detectar el perfil SSO)
  sso_start_url: "https://..."
  sso_region: us-east-1
  profile: ""                # vacío = auto-detectar; o pon un nombre explícito
  ssh_key: "~/.ssh/wb-prod"  # llave Ed25519 (se genera automáticamente)

tunnels:
  - name: "MySQL principal"
    instance_id: "i-0abc..."
    remote_port: 3306
    local_port: 13306
    group: db-c1             # etiqueta de agrupación visual
    type: tcp                # tcp | http

servers:
  - name: "db-c1"
    label: "MySQL / Mongo / Redis"
    instance_id: "i-0abc..."
    tunnel_port: 2202        # puerto local efímero para el túnel SSH
    ssh_user: ubuntu
    private_ip: "172.31.5.49"
```

### Variables de entorno

| Variable | Efecto |
|---|---|
| `LZB_CONFIG` | Ruta alternativa al `config.yml` |
| `LZB_PROFILE` | Fuerza un perfil AWS concreto |

## Uso

```bash
./lzb
```

Al arrancar, la herramienta:
1. Detecta el perfil AWS SSO que corresponde al `account_id` configurado.
2. Valida la sesión SSO; si expiró, abre el navegador para hacer login.
3. Lanza la TUI.

### Teclado

| Tecla | Acción |
|---|---|
| `↑` / `k` | Mover cursor arriba |
| `↓` / `j` | Mover cursor abajo |
| `↵` / `Space` | **Túnel:** toggle abrir/cerrar · **Servidor:** conectar por SSH |
| `d` | Matar el túnel seleccionado |
| `a` | Abrir todos los túneles a la vez |
| `x` | Cerrar todos los túneles |
| `q` / `Ctrl+C` | Salir (cierra todos los túneles antes de salir) |

## Arquitectura

```
main.go                  — pre-auth SSO, lanza la TUI
internal/
  config/config.go       — carga y valida config.yml
  aws/auth.go            — DetectProfile, ValidateSession, Login, CheckGroup
  tunnel/manager.go      — gestiona subprocesos aws ssm start-session (port-forward)
  ssh/connect.go         — EnsureKey, RegisterKey, StartSSHTunnel, BuildSSHCmd
  tui/app.go             — modelo bubbletea (Init / Update / View)
  tui/styles.go          — paleta de colores lipgloss
```

### Flujo de un túnel

```
Toggle(i)
  └─ startProc()
       ├─ killOrphansOnPort()          limpia puertos huérfanos de sesiones anteriores
       ├─ aws ssm start-session        abre el port-forward vía SSM
       └─ goroutine: isPortOpen()      polling cada 400 ms → StatusActive
                                       timeout 30 s → StatusError
```

### Flujo de conexión SSH

```
Enter (servidor)
  └─ prepareSSH()
       ├─ EnsureKey()                  genera ~/.ssh/wb-prod si no existe
       ├─ RegisterKey()                envía la clave pública vía SSM send-command
       ├─ StartSSHTunnel()             aws ssm start-session a puerto 22
       ├─ WaitForPort()                espera hasta 30 s
       └─ tea.ExecProcess(ssh …)       toma el terminal con ssh interactivo
```

## Logs de túneles

Cada túnel escribe su salida en `/tmp/lzb-<puerto>.log`.  
Los túneles SSH escriben en `/tmp/lzb-ssh-<nombre>.log`.

## Dependencias principales

| Paquete | Uso |
|---|---|
| `charmbracelet/bubbletea` | framework TUI |
| `charmbracelet/lipgloss` | estilos de terminal |
| `charmbracelet/bubbles` | spinner |
| `gopkg.in/yaml.v3` | parsing de config.yml |
