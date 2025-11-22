package mocks

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/mantonx/viewra/internal/domain/images"
)

// ImageRepository is a mock implementation of images.Repository for testing.
type ImageRepository struct {
	t      testing.TB
	mu     sync.RWMutex
	imgs   map[int]*images.Image // key: image ID
	nextID int

	// Error injection
	CreateErr              error
	GetErr                 error
	UpdateErr              error
	DeleteErr              error
	DeleteByMediaIDErr     error
	DeleteByEntityErr      error
	ListOrphansErr         error
	ListBySourceErr        error
	GetByFilePathErr       error
	GetAllFileHashesErr    error
	GetByHashErr           error
	HasImagesForEntityErr  error
}

// NewImageRepository creates a new mock image repository.
func NewImageRepository(t testing.TB) *ImageRepository {
	return &ImageRepository{
		t:      t,
		imgs:   make(map[int]*images.Image),
		nextID: 1,
	}
}

// WithImages pre-populates the mock with images.
func (r *ImageRepository) WithImages(items ...*images.Image) *ImageRepository {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, item := range items {
		r.imgs[item.ID] = item
		if item.ID >= r.nextID {
			r.nextID = item.ID + 1
		}
	}
	return r
}

// Implementation of images.Repository interface

func (r *ImageRepository) Create(ctx context.Context, image *images.Image) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if image.ID == 0 {
		image.ID = r.nextID
		r.nextID++
	}

	r.imgs[image.ID] = image
	return nil
}

func (r *ImageRepository) GetByID(ctx context.Context, id int) (*images.Image, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	img, exists := r.imgs[id]
	if !exists {
		return nil, sql.ErrNoRows
	}

	return img, nil
}

func (r *ImageRepository) GetByMediaID(ctx context.Context, mediaID int) ([]*images.Image, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*images.Image
	for _, img := range r.imgs {
		if img.MediaID != nil && *img.MediaID == mediaID {
			result = append(result, img)
		}
	}

	return result, nil
}

func (r *ImageRepository) GetByEntity(ctx context.Context, mediaType images.MediaType, entityID int) ([]*images.Image, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*images.Image
	for _, img := range r.imgs {
		if img.MediaType == mediaType && img.EntityID == entityID {
			result = append(result, img)
		}
	}

	return result, nil
}

func (r *ImageRepository) GetByTypeAndEntity(ctx context.Context, mediaType images.MediaType, entityID int, imageType images.ImageType) (*images.Image, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find highest priority image of this type for this entity
	var best *images.Image
	for _, img := range r.imgs {
		if img.MediaType == mediaType && img.EntityID == entityID && img.ImageType == imageType {
			if best == nil || img.Priority < best.Priority {
				best = img
			}
		}
	}

	if best == nil {
		return nil, sql.ErrNoRows
	}

	return best, nil
}

func (r *ImageRepository) GetByTypeAndMediaID(ctx context.Context, mediaID int, imageType images.ImageType) (*images.Image, error) {
	if r.GetErr != nil {
		return nil, r.GetErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find highest priority image of this type for this media
	var best *images.Image
	for _, img := range r.imgs {
		if img.MediaID != nil && *img.MediaID == mediaID && img.ImageType == imageType {
			if best == nil || img.Priority < best.Priority {
				best = img
			}
		}
	}

	if best == nil {
		return nil, sql.ErrNoRows
	}

	return best, nil
}

func (r *ImageRepository) Update(ctx context.Context, image *images.Image) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.imgs[image.ID]; !exists {
		return sql.ErrNoRows
	}

	r.imgs[image.ID] = image
	return nil
}

func (r *ImageRepository) Delete(ctx context.Context, id int) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.imgs[id]; !exists {
		return sql.ErrNoRows
	}

	delete(r.imgs, id)
	return nil
}

func (r *ImageRepository) DeleteByMediaID(ctx context.Context, mediaID int) error {
	if r.DeleteByMediaIDErr != nil {
		return r.DeleteByMediaIDErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for id, img := range r.imgs {
		if img.MediaID != nil && *img.MediaID == mediaID {
			delete(r.imgs, id)
		}
	}

	return nil
}

func (r *ImageRepository) DeleteByEntity(ctx context.Context, mediaType images.MediaType, entityID int) error {
	if r.DeleteByEntityErr != nil {
		return r.DeleteByEntityErr
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for id, img := range r.imgs {
		if img.MediaType == mediaType && img.EntityID == entityID {
			delete(r.imgs, id)
		}
	}

	return nil
}

func (r *ImageRepository) ListOrphans(ctx context.Context) ([]*images.Image, error) {
	if r.ListOrphansErr != nil {
		return nil, r.ListOrphansErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// In mock, just return empty - file system checks are not mocked
	return []*images.Image{}, nil
}

func (r *ImageRepository) ListBySource(ctx context.Context, sourceType images.SourceType) ([]*images.Image, error) {
	if r.ListBySourceErr != nil {
		return nil, r.ListBySourceErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*images.Image
	for _, img := range r.imgs {
		if img.SourceType == sourceType {
			result = append(result, img)
		}
	}

	return result, nil
}

func (r *ImageRepository) GetByFilePath(ctx context.Context, filePath string) (*images.Image, error) {
	if r.GetByFilePathErr != nil {
		return nil, r.GetByFilePathErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, img := range r.imgs {
		if img.FilePath == filePath {
			return img, nil
		}
	}

	return nil, sql.ErrNoRows
}

func (r *ImageRepository) GetAllFileHashes(ctx context.Context) ([]string, error) {
	if r.GetAllFileHashesErr != nil {
		return nil, r.GetAllFileHashesErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	hashes := make(map[string]bool)
	for _, img := range r.imgs {
		if img.FileHash != nil && *img.FileHash != "" {
			hashes[*img.FileHash] = true
		}
	}

	var result []string
	for hash := range hashes {
		result = append(result, hash)
	}

	return result, nil
}

func (r *ImageRepository) GetByHash(ctx context.Context, hash string) ([]*images.Image, error) {
	if r.GetByHashErr != nil {
		return nil, r.GetByHashErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*images.Image
	for _, img := range r.imgs {
		if img.FileHash != nil && *img.FileHash == hash {
			result = append(result, img)
		}
	}

	return result, nil
}

func (r *ImageRepository) HasImagesForEntity(ctx context.Context, mediaType images.MediaType, entityID int) (bool, error) {
	if r.HasImagesForEntityErr != nil {
		return false, r.HasImagesForEntityErr
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, img := range r.imgs {
		if img.MediaType == mediaType && img.EntityID == entityID {
			return true, nil
		}
	}

	return false, nil
}
