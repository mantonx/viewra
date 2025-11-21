# Port Configuration

## Standard Ports

This project uses consistent ports across all environments:

- **Frontend (Vite)**: `5173`
- **Backend (Go)**: `8080`

## Development Setup

### How It Works

1. Frontend runs on `http://localhost:5173`
2. Backend runs on `http://localhost:8080`
3. All API requests use relative URLs (`/api/*`)
4. Vite's dev server proxies `/api/*` → `http://localhost:8080/api/*`
5. No CORS issues because requests appear to come from same origin

### Configuration Files

**`web/vite.config.ts`**
```typescript
server: {
  port: 5173,
  strictPort: true, // Fail if port in use
  proxy: {
    '/api': {
      target: 'http://localhost:8080',
      changeOrigin: true,
    },
  },
}
```

**`web/src/lib/api/mutator/custom-instance.ts`**
```typescript
const API_BASE_URL = import.meta.env.VITE_API_URL || ''
```

**`web/.env`**
```bash
# Leave VITE_API_URL unset to use default behavior:
# - Development: Uses Vite proxy (relative URLs)
# - Production: Uses /api prefix (same origin)
```

## Troubleshooting

### Port 5173 already in use

Kill existing Vite processes:
```bash
pkill -f vite
# or
lsof -ti:5173 | xargs kill -9
```

### CORS errors

This means requests are bypassing the Vite proxy. Check:

1. Is `VITE_API_URL` set in `.env`? It should be commented out
2. Is the dev server running on port 5173? Check the terminal output
3. Did you hard refresh the browser? (Ctrl+Shift+R)
4. Are you accessing the app at `http://localhost:5173`? (not 5174 or other port)

### Backend not responding

Verify backend is running:
```bash
curl http://localhost:8080/api/health
```

## Production

In production, a reverse proxy (nginx/caddy) should:
1. Serve frontend static files
2. Proxy `/api/*` to the backend service

Frontend will use relative URLs (`/api/*`) which the reverse proxy handles.
