# Database Setup

ViewRA supports both SQLite (default) and PostgreSQL databases.

## CRITICAL: Database Compatibility

**All features must work on BOTH SQLite and PostgreSQL.**

### Development Requirements

1. **Test on both databases** before committing changes
2. **Write portable SQL** - avoid database-specific syntax when possible
3. **Maintain parallel migrations** - one for SQLite, one for PostgreSQL
4. **Document differences** - note any unavoidable database-specific behavior

### SQL Compatibility Guidelines

**Data Types**:
```sql
-- SQLite              PostgreSQL
INTEGER               INTEGER
TEXT                  TEXT
REAL                  DOUBLE PRECISION
DATETIME              TIMESTAMP
BOOLEAN (0/1)         BOOLEAN (true/false)
```

**Auto-Increment**:
```sql
-- SQLite
id INTEGER PRIMARY KEY AUTOINCREMENT

-- PostgreSQL  
id SERIAL PRIMARY KEY
```

**Timestamps**:
```sql
-- SQLite
created_at DATETIME DEFAULT CURRENT_TIMESTAMP

-- PostgreSQL
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
```

**Checks and Constraints**:
- Both support `CHECK` constraints
- Both support `UNIQUE` constraints
- Both support foreign keys (SQLite requires `PRAGMA foreign_keys = ON`)

---

## SQLite (Default)

SQLite is used by default for easier development and single-user deployments.

```bash
# Set in .env (or use defaults)
DB_DRIVER=sqlite
DB_PATH=data/viewra.db

# Run migrations
make migrate-up
```

**Advantages:**
- Zero configuration required
- Perfect for home/personal use
- No separate database server needed
- Automatic backups via file copy

## PostgreSQL (Production)

For multi-user deployments or better performance under load, use PostgreSQL.

### Setup PostgreSQL

```bash
# Install PostgreSQL (Ubuntu/Debian)
sudo apt install postgresql postgresql-contrib

# Create database and user
sudo -u postgres psql
CREATE DATABASE viewra;
CREATE USER viewra WITH PASSWORD 'your_password_here';
GRANT ALL PRIVILEGES ON DATABASE viewra TO viewra;
\q
```

### Configure ViewRA

```bash
# Update .env
DB_DRIVER=postgres
DB_HOST=localhost
DB_PORT=5432
DB_USER=viewra
DB_PASSWORD=your_password_here
DB_NAME=viewra
DB_SSL_MODE=disable
```

### Run Migrations

```bash
# PostgreSQL migrations are in migrations/postgres/
migrate -path migrations/postgres -database "postgres://viewra:password@localhost:5432/viewra?sslmode=disable" up
```

**Advantages:**
- Better performance for concurrent users
- Advanced indexing and query optimization
- Better suited for Docker deployments
- Production-ready ACID compliance

## Migration Commands

```bash
# SQLite
migrate -path migrations -database "sqlite3://data/viewra.db" up
migrate -path migrations -database "sqlite3://data/viewra.db" down 1

# PostgreSQL  
migrate -path migrations/postgres -database "postgres://user:pass@host:5432/dbname?sslmode=disable" up
migrate -path migrations/postgres -database "postgres://user:pass@host:5432/dbname?sslmode=disable" down 1
```

## Switching Databases

You can switch between SQLite and PostgreSQL by changing the `DB_DRIVER` environment variable. The application will automatically use the correct driver and connection string.

**Note:** Data is not automatically migrated between databases. You'll need to export/import data manually if switching databases with existing data.
