# Schema-Driven Plugin Settings System

## Overview

Create a generic, schema-driven action system for plugin settings. Plugins define UI actions through `x-viewra-actions` in their JSON Schema. The core renders these actions in a tabbed interface without any plugin-specific code.

### Goals

1. **Unify plugin API** - All plugins use `/api/plugins/{pluginId}/...` endpoints
2. **Expose provider ID from plugins** - Plugins declare their provider ID for lookup
3. **Remove redundant AI provider endpoints** - Clean break, no deprecated aliases
4. **Create generic `PluginSettingsForm`** - Works for any plugin type
5. **Schema-driven actions** - `x-viewra-actions` renders generic action components

### Example Result

Tab UI for Ollama provider:
```
[Settings]  [Models (5)]
```

---

## Part 1: Schema Design

### Action Types

| Type | Description |
|------|-------------|
| `list` | Display a list of items with optional per-item actions |
| `create` | Add a new item, possibly with streaming progress |
| `delete` | Remove an item with confirmation |
| `test` | Run a check and show result |
| `form` | Display a mini-form and submit it |

### Complete Ollama Schema Example

```json
{
  "type": "object",
  "title": "Ollama Settings",
  "properties": {
    "base_url": {
      "type": "string",
      "title": "Server URL",
      "description": "URL of the Ollama server",
      "default": "http://localhost:11434"
    },
    "embedding_model": {
      "type": "string",
      "title": "Embedding Model",
      "description": "Model to use for generating embeddings"
    },
    "chat_model": {
      "type": "string",
      "title": "Chat Model",
      "description": "Model to use for chat completions"
    }
  },
  "x-viewra-actions": [
    {
      "id": "installed-models",
      "type": "list",
      "title": "Installed Models",
      "tabTitle": "Models",
      "source": {
        "endpoint": "/models"
      },
      "display": {
        "primaryField": "name",
        "secondaryField": "description",
        "badges": [
          { "field": "isEmbedding", "label": "Embedding", "color": "blue" },
          { "field": "isChat", "label": "Chat", "color": "purple" }
        ],
        "metadata": ["size"]
      },
      "itemActions": [
        {
          "id": "delete",
          "type": "delete",
          "icon": "trash",
          "endpoint": "/models/:id",
          "confirm": {
            "title": "Delete Model",
            "message": "This will remove the model from Ollama."
          }
        }
      ],
      "emptyState": {
        "title": "No models installed",
        "description": "Pull a model to get started",
        "showCreate": "pull-model"
      }
    },
    {
      "id": "pull-model",
      "type": "create",
      "title": "Pull New Model",
      "endpoint": "/models/pull",
      "streaming": true,
      "input": {
        "field": "model",
        "placeholder": "e.g., llama3.2, nomic-embed-text",
        "helpLink": {
          "url": "https://ollama.com/library",
          "text": "Browse Ollama library"
        }
      },
      "progress": {
        "statusField": "status",
        "totalField": "total",
        "completedField": "completed",
        "doneField": "done",
        "errorField": "error"
      },
      "onSuccess": {
        "refresh": "installed-models"
      }
    }
  ]
}
```

### Tab Title Logic

1. If a `list` action has `tabTitle`, use it + count: `"Models (5)"`
2. Else if a `list` action exists, extract from `title`: `"Installed Models"` → `"Models (5)"`
3. Else fall back to `"Actions"`

---

## Part 2: API Unification

### Current State

| Endpoint | Exists? | Purpose |
|----------|---------|---------|
| `GET /api/plugins` | ✅ | List all plugins |
| `GET /api/plugins/:id` | ✅ | Get plugin details |
| `GET /api/plugins/:id/settings` | ✅ | Get schema + values |
| `PUT /api/plugins/:id/settings` | ✅ | Update settings |
| `GET /api/plugins/:id/health` | ✅ | Health check |
| `POST /api/plugins/:id/enable` | ✅ | Enable plugin |
| `POST /api/plugins/:id/disable` | ✅ | Disable plugin |
| `POST /api/plugins/:id/restart` | ✅ | Restart plugin |
| `GET /api/plugins/:id/logs` | ✅ | Get logs |
| `/api/plugins/:id/*path` | ❌ | Plugin custom routes (MISSING) |
| `/api/plugin-routes/:id/*path` | ✅ | Plugin custom routes (wrong URL) |

### Target State

All plugin endpoints unified under `/api/plugins`:

```
GET    /api/plugins                      # List all plugins
GET    /api/plugins/:id                  # Get plugin details  
GET    /api/plugins/:id/settings         # Get schema + values
PUT    /api/plugins/:id/settings         # Update settings
GET    /api/plugins/:id/health           # Health check
POST   /api/plugins/:id/enable           # Enable plugin
POST   /api/plugins/:id/disable          # Disable plugin
POST   /api/plugins/:id/restart          # Restart plugin
GET    /api/plugins/:id/logs             # Get logs
*      /api/plugins/:id/*path            # Plugin custom routes
```

### AI Settings Routes

Keep these AI-specific routes:

```
GET /api/settings/ai                     # Get AI feature settings
PUT /api/settings/ai                     # Update AI feature settings
GET /api/settings/ai/providers           # List available providers with capabilities
GET /api/settings/ai/models/recommended  # Model recommendations
GET /api/settings/ai/models/recommended/chat
```

Remove these (replaced by unified plugin routes):

```
GET /api/settings/ai/providers/:provider/schema      # Use /api/plugins/:id/settings
PUT /api/settings/ai/providers/:provider/configure   # Use /api/plugins/:id/settings
GET /api/settings/ai/providers/:provider/models      # Use plugin custom routes
POST /api/settings/ai/providers/:provider/test       # Use /api/plugins/:id/health
```

---

## Part 3: Backend Changes

### 3.1 Fix Plugin Custom Routes

**File:** `internal/api/routes/plugins.go`

Add wildcard route for plugin custom endpoints at the end:

```go
// Plugin custom routes (must be last to not override specific routes)
plugins.Any("/:id/*path", proxy.HandlePluginRoute)
```

**File:** `internal/api/server.go`

Fix comment at line 253 and remove the `/api/plugin-routes` registration (line 266).

### 3.2 Add Provider ID to Plugin System

**File:** `internal/application/plugins/types.go`

Add to `PluginSummary`:

```go
type PluginSummary struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    // ... existing fields ...
    ProviderID  string   `json:"provider_id,omitempty"` // e.g., "ollama" for provider-ollama
}
```

**File:** `internal/application/plugins/service.go`

In `List()` and `Get()`, populate `ProviderID` from the plugin's provider capabilities.

### 3.3 Add Size to Domain ModelInfo

**File:** `internal/domain/ai/provider.go`

```go
type ModelInfo struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Description string   `json:"description"`
    Size        string   `json:"size,omitempty"` // NEW: e.g., "4.7 GB"
    // ... rest unchanged
}
```

**File:** `internal/api/handlers/ai_settings.go`

Map `Size: m.Size` from protobuf in the handler.

### 3.4 Remove AI Provider Routes

**File:** `internal/api/routes/ai_settings.go`

Remove:
```go
ai.GET("/providers/:provider/models", h.ListModels)
ai.POST("/providers/:provider/test", h.TestConnection)
ai.GET("/providers/:provider/schema", h.GetProviderSchema)
ai.PUT("/providers/:provider/configure", h.ConfigureProvider)
```

**File:** `internal/api/handlers/ai_settings.go`

Remove handler methods:
- `ListModels`
- `TestConnection`
- `GetProviderSchema`
- `ConfigureProvider`

### 3.5 Update Ollama Plugin Schema

**File:** `plugins/provider-ollama/internal/plugin.go`

Update `GetSettingsSchema()` to return schema with `x-viewra-actions` (see example above).

### 3.6 Add GET /models Route to Ollama Plugin

**File:** `plugins/provider-ollama/internal/plugin.go`

Update `GetRoutes()`:
```go
{
    Path:        "/models",
    Methods:     []string{"GET"},
    AdminOnly:   false,
    Description: "List installed models",
},
```

Update `HandleHTTP()` to handle `GET /models`:
```go
if req.Method == "GET" && req.Path == "/models" {
    modelList, err := p.provider.ListModels(ctx, &pluginv1.Empty{})
    if err != nil {
        return jsonError(500, err.Error())
    }
    return jsonResponse(200, map[string]any{
        "models": modelList.Models,
    })
}
```

---

## Part 4: Frontend Changes

### 4.1 TypeScript Types

**File:** `web/src/lib/types/schema-actions.ts` (CREATE)

```typescript
// Badge configuration for list items
export type BadgeColor = 'blue' | 'purple' | 'green' | 'red' | 'yellow' | 'gray'

export type BadgeConfig = {
  field: string
  label: string
  color: BadgeColor
  whenTrue?: boolean
}

export type ListDisplayConfig = {
  primaryField: string
  secondaryField?: string
  badges?: BadgeConfig[]
  metadata?: string[]
}

export type EmptyStateConfig = {
  title: string
  description?: string
  showCreate?: string
}

export type ConfirmConfig = {
  title?: string
  message: string
}

export type InputConfig = {
  field: string
  placeholder?: string
  helpLink?: {
    url: string
    text: string
  }
}

export type ProgressConfig = {
  statusField: string
  totalField?: string
  completedField?: string
  doneField: string
  errorField?: string
}

export type OnSuccessConfig = {
  refresh?: string
  message?: string
}

export type SourceConfig = {
  endpoint: string
  method?: 'GET' | 'POST'
}

// Item actions
export type ItemDeleteAction = {
  id: string
  type: 'delete'
  icon?: string
  endpoint: string
  confirm?: ConfirmConfig
}

export type ItemTestAction = {
  id: string
  type: 'test'
  icon?: string
  endpoint: string
}

export type ItemAction = ItemDeleteAction | ItemTestAction

// Top-level actions
export type ListAction = {
  id: string
  type: 'list'
  title: string
  tabTitle?: string
  source: SourceConfig
  display: ListDisplayConfig
  itemActions?: ItemAction[]
  emptyState?: EmptyStateConfig
}

export type CreateAction = {
  id: string
  type: 'create'
  title: string
  endpoint: string
  streaming?: boolean
  input: InputConfig
  progress?: ProgressConfig
  onSuccess?: OnSuccessConfig
}

export type DeleteAction = {
  id: string
  type: 'delete'
  title: string
  endpoint: string
  confirm?: ConfirmConfig
  onSuccess?: OnSuccessConfig
}

export type TestAction = {
  id: string
  type: 'test'
  title: string
  endpoint: string
  successMessage?: string
  errorMessage?: string
}

export type FormAction = {
  id: string
  type: 'form'
  title: string
  endpoint: string
  schema: Record<string, unknown>
  submitLabel?: string
  onSuccess?: OnSuccessConfig
}

export type SchemaAction = ListAction | CreateAction | DeleteAction | TestAction | FormAction

export type ExtendedSchema = {
  'x-viewra-actions'?: SchemaAction[]
  [key: string]: unknown
}

// Type guards
export const isListAction = (action: SchemaAction): action is ListAction => action.type === 'list'
export const isCreateAction = (action: SchemaAction): action is CreateAction => action.type === 'create'
export const isDeleteAction = (action: SchemaAction): action is DeleteAction => action.type === 'delete'
export const isTestAction = (action: SchemaAction): action is TestAction => action.type === 'test'
export const isFormAction = (action: SchemaAction): action is FormAction => action.type === 'form'
```

### 4.2 Component Structure

```
web/src/components/settings/
├── PluginSettingsForm/
│   ├── index.ts
│   ├── PluginSettingsForm.tsx      # Main component
│   └── PluginSettingsTabs.tsx      # Tab UI with dynamic labels
├── SchemaActions/
│   ├── index.ts
│   ├── SchemaActions.tsx           # Container - iterates actions
│   ├── ActionList.tsx              # type: "list"
│   ├── ActionCreate.tsx            # type: "create"
│   ├── ActionDelete.tsx            # type: "delete"
│   ├── ActionTest.tsx              # type: "test"
│   └── ActionForm.tsx              # type: "form"
└── index.ts                        # Updated exports
```

### 4.3 Component Specifications

#### PluginSettingsForm

```typescript
type PluginSettingsFormProps = {
  pluginId: string
  className?: string
}
```

- Uses `GET /api/plugins/{pluginId}/settings` for schema + values
- Uses `PUT /api/plugins/{pluginId}/settings` to save
- Renders "Settings" tab with JSON Schema form
- Renders actions tab if `x-viewra-actions` present
- Actions tab label from first list action's `tabTitle` + live count

#### SchemaActions

```typescript
type SchemaActionsProps = {
  pluginId: string
  actions: SchemaAction[]
}
```

- Maps action types to components
- Manages refresh callbacks between actions

#### ActionList

```typescript
type ActionListProps = {
  pluginId: string
  config: ListAction
  refreshKey?: number
  onCountChange?: (count: number) => void
}
```

- Fetches from `config.source.endpoint`
- Renders items with primary/secondary fields, badges, metadata
- Renders per-item actions
- Shows empty state with prominent create if `showCreate` set

#### ActionCreate

```typescript
type ActionCreateProps = {
  pluginId: string
  config: CreateAction
  onSuccess?: () => void
  prominent?: boolean
}
```

- Renders input based on `config.input`
- POSTs to `config.endpoint`
- Handles SSE streaming using `config.progress` mappings
- Shows progress bar

#### ActionDelete

```typescript
type ActionDeleteProps = {
  pluginId: string
  config: DeleteAction
  onSuccess?: () => void
}
```

- Button with confirmation modal
- DELETEs to `config.endpoint`

#### ActionTest

```typescript
type ActionTestProps = {
  pluginId: string
  config: TestAction
}
```

- Button that POSTs to `config.endpoint`
- Shows success/error toast

#### ActionForm

```typescript
type ActionFormProps = {
  pluginId: string
  config: FormAction
  onSuccess?: () => void
}
```

- Renders JSON Schema form from `config.schema`
- POSTs to `config.endpoint`

### 4.4 Update AI Settings Page

**File:** `web/src/routes/_layout/settings.ai.tsx`

1. Import `PluginSettingsForm` instead of `ProviderSettingsForm`
2. Map provider ID to plugin ID: `ollama` → `provider-ollama`
3. Use `GET /api/plugins/{pluginId}/health` for connection testing

```typescript
const getPluginId = (providerId: string) => `provider-${providerId}`

// In ProviderCard:
<PluginSettingsForm pluginId={getPluginId(provider.type)} />
```

### 4.5 Delete Old Components

| Directory | Action |
|-----------|--------|
| `web/src/components/settings/ProviderSettingsForm/` | DELETE |
| `web/src/components/settings/OllamaModelManager/` | DELETE |

---

## Part 5: File Changes Summary

### Backend (8 files)

| File | Change |
|------|--------|
| `internal/api/server.go` | Fix comment, remove plugin-routes registration |
| `internal/api/routes/plugins.go` | Add wildcard route for plugin custom endpoints |
| `internal/api/routes/ai_settings.go` | Remove provider-specific routes |
| `internal/api/handlers/ai_settings.go` | Remove provider-specific handlers |
| `internal/application/plugins/types.go` | Add `ProviderID` to `PluginSummary` |
| `internal/domain/ai/provider.go` | Add `Size` to `ModelInfo` |
| `plugins/provider-ollama/internal/plugin.go` | Add `x-viewra-actions`, add GET /models route |
| `plugins/provider-ollama/internal/provider.go` | Ensure ListModels returns size (already does) |

### Frontend (15 files)

| File | Change |
|------|--------|
| `web/src/lib/types/schema-actions.ts` | CREATE |
| `web/src/components/settings/SchemaActions/index.ts` | CREATE |
| `web/src/components/settings/SchemaActions/SchemaActions.tsx` | CREATE |
| `web/src/components/settings/SchemaActions/ActionList.tsx` | CREATE |
| `web/src/components/settings/SchemaActions/ActionCreate.tsx` | CREATE |
| `web/src/components/settings/SchemaActions/ActionDelete.tsx` | CREATE |
| `web/src/components/settings/SchemaActions/ActionTest.tsx` | CREATE |
| `web/src/components/settings/SchemaActions/ActionForm.tsx` | CREATE |
| `web/src/components/settings/PluginSettingsForm/index.ts` | CREATE |
| `web/src/components/settings/PluginSettingsForm/PluginSettingsForm.tsx` | CREATE |
| `web/src/components/settings/PluginSettingsForm/PluginSettingsTabs.tsx` | CREATE |
| `web/src/components/settings/index.ts` | MODIFY |
| `web/src/routes/_layout/settings.ai.tsx` | MODIFY |
| `web/src/components/settings/ProviderSettingsForm/` | DELETE (directory) |
| `web/src/components/settings/OllamaModelManager/` | DELETE (directory) |

---

## Part 6: Implementation Order

1. **Backend: Plugin routes** - Add wildcard, fix server.go
2. **Backend: ProviderID** - Add to plugin types
3. **Backend: ModelInfo Size** - Add field, map in handler
4. **Backend: Remove AI provider routes** - Clean up routes and handlers
5. **Backend: Ollama plugin** - Add schema actions, GET /models route
6. **Regenerate** - `make api-client-gen`
7. **Frontend: Types** - `schema-actions.ts`
8. **Frontend: Action components** - ActionTest → ActionDelete → ActionCreate → ActionList → ActionForm
9. **Frontend: SchemaActions** - Container
10. **Frontend: PluginSettingsForm** - With tabs
11. **Frontend: AI settings page** - Update to use new components
12. **Frontend: Cleanup** - Delete old components
13. **Build & Test** - `make build-plugins && cd web && npm run build`

---

## Part 7: Testing Checklist

- [ ] `GET /api/plugins` returns plugins with `provider_id` field
- [ ] `GET /api/plugins/provider-ollama/settings` returns schema with `x-viewra-actions`
- [ ] `PUT /api/plugins/provider-ollama/settings` configures and persists
- [ ] `GET /api/plugins/provider-ollama/health` returns health status
- [ ] `GET /api/plugins/provider-ollama/models` returns model list
- [ ] `POST /api/plugins/provider-ollama/models/pull` streams progress
- [ ] `DELETE /api/plugins/provider-ollama/models/:id` deletes model
- [ ] AI settings page loads providers
- [ ] AI settings page shows Settings tab with form
- [ ] AI settings page shows Models tab with count
- [ ] Models tab lists installed models with badges
- [ ] Can pull new model with progress
- [ ] Can delete model with confirmation
- [ ] Empty state shows pull form prominently

---

## Part 8: Future Extensions

This schema-driven system can be extended for other plugins:

### Example: Cache Manager Plugin

```json
{
  "x-viewra-actions": [
    {
      "id": "cache-entries",
      "type": "list",
      "title": "Cached Items",
      "source": { "endpoint": "/cache" },
      "display": {
        "primaryField": "key",
        "secondaryField": "createdAt",
        "metadata": ["size"]
      },
      "itemActions": [
        {
          "id": "delete",
          "type": "delete",
          "endpoint": "/cache/:key",
          "confirm": { "message": "Remove this cached item?" }
        }
      ]
    },
    {
      "id": "clear-all",
      "type": "delete",
      "title": "Clear All Cache",
      "endpoint": "/cache",
      "confirm": { "message": "This will remove all cached data." }
    }
  ]
}
```

### Example: Device Manager Plugin

```json
{
  "x-viewra-actions": [
    {
      "id": "devices",
      "type": "list",
      "title": "Connected Devices",
      "source": { "endpoint": "/devices" },
      "display": {
        "primaryField": "name",
        "secondaryField": "ip",
        "badges": [
          { "field": "online", "label": "Online", "color": "green", "whenTrue": true }
        ]
      },
      "itemActions": [
        {
          "id": "test",
          "type": "test",
          "icon": "refresh",
          "endpoint": "/devices/:id/ping"
        },
        {
          "id": "forget",
          "type": "delete",
          "endpoint": "/devices/:id",
          "confirm": { "message": "Forget this device?" }
        }
      ]
    },
    {
      "id": "discover",
      "type": "create",
      "title": "Discover Devices",
      "endpoint": "/devices/discover",
      "streaming": true,
      "input": {
        "field": "subnet",
        "placeholder": "192.168.1.0/24"
      }
    }
  ]
}
```
