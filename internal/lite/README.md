# Crush Lite - CLI Minimalista para Crush

`crush lite` es una interfaz CLI minimalista diseñada para funcionar tanto con humanos como con agentes de IA externos.

## Características

- **Salida limpia y legible** para humanos
- **Salida JSON estructurada** para integración con agentes externos
- **Manejo interactivo de permisos** (confirmaciones y/n/a)
- **Soporte completo de streaming** de respuestas del agente
- **Continuación de sesiones** para flujos de trabajo largos

## Uso

### Modo Interactivo (default)

```bash
crush lite "describe este proyecto"
```

### Modo Plain (sin colores)

```bash
crush lite --plain "lista todos los archivos"
```

### Modo Agente (JSON para IA externa)

```bash
crush lite --agent "explica este error" | jq
```

### Auto-permisos

```bash
# Saltar preguntas de permisos (auto-permitir)
crush lite --skip-permissions "arreglar los tests"

# Yolo mode (permitir todo, usar con precaución)
crush lite --yolo "eliminar todos los archivos .tmp"
```

### Continuar Sesiones

```bash
# Continuar una sesión específica
crush lite --session <id> "continuar desde donde quedamos"

# Continuar la última sesión
crush lite --continue "seguir con el desarrollo"
```

### Integración con Agentes Externos

El modo agente (`--agent`)输出 JSON estructurado:

```json
{"type": "text", "content": "Voy a analizar...", "time": 1713000000}
{"type": "tool_call", "content": "[Bash] ls -la", "time": 1713000001}
{"type": "tool_result", "content": "✓ archivo1.go\n  archivo2.go", "time": 1713000002}
{"type": "permission_request", "content": "{\"id\":\"...\",\"tool_name\":\"Bash\",\"description\":\"Eliminar archivos\"}", "time": 1713000003}
```

## Arquitectura

```
crush lite
    │
    ├─> Cliente HTTP → Crush Server (SSE)
    │                    │
    │                    ├─> Workspace
    │                    ├─> Agent (LLM)
    │                    └─> Permissions
    │
    └─> Renderer (human/plain/agent)
            │
            ├─> Formateo de mensajes
            ├─> Formateo de herramientas
            └─> Permisos interactivos
```

## Flags Disponibles

| Flag | Descripción |
|------|-------------|
| `--mode human\|plain\|agent` | Modo de salida (default: human) |
| `--plain` | Salida sin colores ni formatos |
| `--agent` | Salida JSON para integración con agentes |
| `--quiet, -q` | Ocultar spinner y salida no esencial |
| `--verbose, -v` | Mostrar logs detallados |
| `--skip-permissions` | Saltar preguntas de permisos |
| `--yolo, -y` | Permitir todas las acciones (peligroso) |
| `--session, -s` | Continuar sesión por ID |
| `--continue, -C` | Continuar la última sesión |

## Eventos JSON (modo agente)

| Tipo | Descripción |
|------|-------------|
| `text` | Texto de respuesta del asistente |
| `thinking` | Razonamiento interno del modelo |
| `tool_call` | Llamada a herramienta (bash, edit, view, etc.) |
| `tool_result` | Resultado de una herramienta |
| `permission_request` | Solicitud de confirmación |
| `permission` | Notificación de resultado de permiso |
| `user` | Echo del mensaje del usuario |
| `help` | Texto de ayuda |

## Ejemplos de Integración

### Con jq para filtrar salida

```bash
crush lite --agent "mostrar los archivos modificados" | jq '.content'
```

### Pipe a otro proceso

```bash
crush lite --plain "generar changelog" | grep -E "feat|fix|docs"
```

### Script de automatización

```bash
#!/bin/bash
RESPONSE=$(crush lite --agent --skip-permissions "fix: corregir bug en login")
echo "$RESPONSE" | jq -r '.content'
```

## Comparación con otros modos

| Modo | Uso |
|-------|-----|
| `crush` (TUI) | Uso interactivo humano con interfaz visual completa |
| `crush run` | Ejecución no-interactiva de un solo prompt |
| `crush lite` | CLI minimalista para humanos y agentes |

## Notas

- Requiere que el servidor de Crush esté corriendo (`crush server`)
- En modo local (sin servidor), usa `CRUSH_CLIENT_SERVER=0` (experimental)
