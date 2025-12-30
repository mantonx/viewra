// Package host provides vector storage operations for plugins.
// This file imports required drivers and the functionality is split across:
// - storage_vector_store.go: Store operations (VectorStoreEmbedding, VectorStoreBatch)
// - storage_vector_search.go: Search operations (VectorSearch, VectorSearchText)
// - storage_vector_crud.go: CRUD operations (VectorGet, VectorDelete, VectorDeleteByType, VectorCount)
// - storage_vector_helpers.go: Helper functions and table management
package host

import (
	_ "github.com/mattn/go-sqlite3" // SQLite driver (required for sqlite-vec)
)
