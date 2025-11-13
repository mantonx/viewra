# Quick Reference: Preventing Incomplete Implementations

**1-page cheat sheet for building complete features**

---

## The Golden Rule

> A feature isn't done until data flows correctly through ALL layers and an integration test proves it.

---

## Before You Code

```bash
# Check for incomplete implementations
make audit

# Plan your vertical slice
# Database → Domain → Repository → Use Case → API → UI
```

---

## Repository Anti-Pattern (DON'T)

```go
// ❌ BAD - Data is lost!
params := CreateParams{
    Title:    m.Title,
    FileHash: sql.NullString{},      // Empty!
    Codec:    sql.NullString{},      // Empty!
    Bitrate:  sql.NullInt64{},       // Empty!
}
```

## Repository Correct Pattern (DO)

```go
// ✅ GOOD - All data preserved
params := CreateParams{
    Title:    m.Title,
    FileHash: common.NullString(m.Hash),
    Codec:    common.NullString(m.VideoCodec),
    Bitrate:  common.NullInt64(m.Bitrate),
}
```

---

## Integration Test Template

```go
func TestFeature_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // 1. Real database
    db := setupTestDatabase(t)
    defer db.Close()

    // 2. Real dependencies
    repo := NewRepository(db, "sqlite")
    useCase := NewUseCase(repo)

    // 3. Execute full workflow
    result, err := useCase.Execute(ctx, input)
    require.NoError(t, err)

    // 4. Verify output
    require.Equal(t, expected, result.Field)

    // 5. Verify database
    saved, err := repo.GetByID(ctx, result.ID)
    require.NoError(t, err)
    require.Equal(t, expected, saved.Field)
    // ⚠️  Verify ALL fields!
}
```

---

## Definition of Done Checklist

**Database Layer**:
- [ ] Migration created (up/down)
- [ ] sqlc queries written
- [ ] `make sqlc` successful

**Repository Layer**:
- [ ] ALL domain fields mapped in Create
- [ ] ALL domain fields mapped in Update
- [ ] No empty `sql.Null{}`
- [ ] Both Postgres & SQLite

**Testing**:
- [ ] Integration test exists
- [ ] Test verifies ALL fields
- [ ] `make test` passes

**Verification**:
- [ ] Manual test with real data
- [ ] `make audit` shows improvement
- [ ] No TODO in production code

---

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Empty `sql.NullString{}` | Map actual field: `common.NullString(m.Field)` |
| No integration test | Write one that proves end-to-end flow |
| No-op in production | Implement real version or remove dependency |
| "Will implement later" | Either implement now or create ticket |
| Testing with mocks only | Use real database in integration tests |

---

## Quick Commands

```bash
# Check for incomplete implementations
make audit

# Run all tests
make test

# Check if field is mapped
grep -r "FieldName" internal/infrastructure/persistence/

# Generate code
make sqlc
make swagger-gen
cd web && npm run generate:api

# Create migration
make migrate-create NAME=feature_name
```

---

## When to Ask Questions

- See `sql.NullString{}`? → Should this map a field?
- See `// TODO`? → Can I complete this now?
- See no-op? → Can I implement real version?
- No integration test? → Should I write one?
- Not sure if complete? → Run through checklist above

---

## Documentation Index

- **[INCOMPLETE_IMPLEMENTATIONS.md](./INCOMPLETE_IMPLEMENTATIONS.md)** - Current issues and priorities
- **[This Document]** - Complete development workflow guide

---

## The Vertical Slice Approach

**DON'T** (Horizontal):
```
Week 1: All domain models ✓
Week 2: All repositories (half done)
Week 3: All APIs (not started)
Result: Nothing works
```

**DO** (Vertical):
```
Week 1: Library CRUD (all layers) ✓
Week 2: Media scanning (all layers) ✓
Week 3: Watch progress (all layers) ✓
Result: Each feature works completely
```

---

## Remember

✓ Map every field in repository layer
✓ Write integration tests for database features
✓ No no-op implementations in production
✓ Run `make audit` before committing
✓ Follow the Definition of Done

**Every field. Every layer. Every time.**

## Testing Strategy

See **[TESTING.md](./TESTING.md)** for complete testing guidelines and current coverage.

### Quick Test Commands

```bash
# Run all tests
make test

# Run with coverage
go test -v -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out

# Coverage summary
go tool cover -func=coverage.out | grep total
```
