# Database Configuration

## Database Path Configuration

Viewra uses a **single SQLite database** located at:

```
./viewra-data/viewra.db
```

### Configuration Priority

The database path is determined in this order:

1. **Environment Variable**: `VIEWRA_DATABASE_PATH`
2. **Docker Compose Default**: `/app/viewra-data/viewra.db`
3. **Config File**: Set via `database.database_path` in `viewra.yaml`
4. **Code Default**: `{data_dir}/viewra.db`

### Environment Variables

```bash
# Primary database configuration
VIEWRA_DATA_DIR=/app/viewra-data
VIEWRA_DATABASE_PATH=/app/viewra-data/viewra.db

# Database type (always sqlite for Viewra)
DATABASE_TYPE=sqlite
```

### Docker Compose Volumes

The database is mounted via:

```yaml
volumes:
  - ./viewra-data:/app/viewra-data
```

## Preventing Multiple Databases

### ✅ Correct Configuration

- **Only one database file**: `./viewra-data/viewra.db`
- **Consistent paths** across all components
- **Shared database connection** for all plugins

### ❌ Common Issues to Avoid

1. **Hardcoded paths** in plugin code
2. **Multiple database files** in different directories
3. **Inconsistent environment variables**
4. **Test databases** not cleaned up

### Cleanup

Use the cleanup script to remove redundant databases:

```bash
./scripts/cleanup-databases.sh
```

## Plugin Database Integration

### For Plugin Developers

```go
// ✅ Use the shared database connection
func (p *Plugin) Initialize(ctx *plugins.PluginContext) error {
    // Get shared database connection
    p.db = ctx.DB.(*gorm.DB)
    p.dbURL = ctx.DatabaseURL  // For migrations
    return nil
}

// ❌ Don't create separate database connections
// Don't do this:
func (p *Plugin) Initialize(ctx *plugins.PluginContext) error {
    db, err := gorm.Open(sqlite.Open("my-plugin.db"), &gorm.Config{})
    // This creates a separate database!
}
```

### Migration Handling

Plugins should use the provided connection string for migrations:

```go
func (p *Plugin) Migrate(connectionString string) error {
    // Use the provided connection string
    db, err := t.connectToDatabase(connectionString)
    if err != nil {
        return err
    }

    // Migrate your tables to the SHARED database
    return db.AutoMigrate(&MyPluginTable{})
}
```

## Database Tools

### SQLite Web Interface

Access the database via web interface:

```bash
docker-compose up sqliteweb
# Visit: http://localhost:8081
```

### Direct SQLite Access

```bash
# Via Docker
docker-compose exec backend sqlite3 /app/viewra-data/viewra.db

# Direct access
sqlite3 ./viewra-data/viewra.db
```

## Troubleshooting

### Multiple Database Files

If you find multiple `.db` files:

1. Run the cleanup script: `./scripts/cleanup-databases.sh`
2. Check for hardcoded paths in custom code
3. Verify environment variables are consistent

### Plugin Database Issues

1. **Check plugin uses shared connection**: Verify plugin gets DB from `ctx.DB`
2. **Check migration path**: Ensure plugin migrations use provided connection string
3. **Check table names**: Verify no table name conflicts between plugins

### Performance Issues

1. **Check connection pool**: Use `database.LogConnectionStats()`
2. **Monitor WAL file size**: SQLite WAL should auto-checkpoint
3. **Check disk space**: Ensure adequate space in `./viewra-data/`
