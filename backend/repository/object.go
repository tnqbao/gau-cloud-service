package repository

import (
	"strings"

	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-orchestrator/entity"
	"gorm.io/gorm"
)

type ObjectRepository struct {
	db *gorm.DB
}

func NewObjectRepository(db *gorm.DB) *ObjectRepository {
	return &ObjectRepository{db: db}
}

func (r *ObjectRepository) Create(object *entity.Object) error {
	return r.db.Create(object).Error
}

func (r *ObjectRepository) FindByID(id uuid.UUID) (*entity.Object, error) {
	var object entity.Object
	err := r.db.Where("id = ?", id).First(&object).Error
	if err != nil {
		return nil, err
	}
	return &object, nil
}

func (r *ObjectRepository) FindByBucketID(bucketID uuid.UUID) ([]entity.Object, error) {
	var objects []entity.Object
	err := r.db.Where("bucket_id = ?", bucketID).Find(&objects).Error
	if err != nil {
		return nil, err
	}
	return objects, nil
}

func (r *ObjectRepository) FindByFileHash(fileHash string) (*entity.Object, error) {
	var object entity.Object
	err := r.db.Where("file_hash = ?", fileHash).First(&object).Error
	if err != nil {
		return nil, err
	}
	return &object, nil
}

func (r *ObjectRepository) FindByBucketIDAndPath(bucketID uuid.UUID, parentPath string) ([]entity.Object, error) {
	var objects []entity.Object
	err := r.db.Where("bucket_id = ? AND parent_path = ?", bucketID, parentPath).Find(&objects).Error
	if err != nil {
		return nil, err
	}
	return objects, nil
}

// FindFoldersByBucketIDAndPath finds distinct folder names at the given path level
func (r *ObjectRepository) FindFoldersByBucketIDAndPath(bucketID uuid.UUID, parentPath string) ([]string, error) {
	var allPaths []string

	// Get all distinct parent_paths that are "deeper" than current path
	if parentPath == "" {
		// Root level: get all non-empty parent_paths
		err := r.db.Model(&entity.Object{}).
			Where("bucket_id = ? AND parent_path != '' AND parent_path IS NOT NULL", bucketID).
			Distinct("parent_path").
			Pluck("parent_path", &allPaths).Error
		if err != nil {
			return nil, err
		}
	} else {
		// Nested level: get parent_paths that start with parentPath/
		prefix := parentPath + "/"
		err := r.db.Model(&entity.Object{}).
			Where("bucket_id = ? AND parent_path LIKE ?", bucketID, prefix+"%").
			Distinct("parent_path").
			Pluck("parent_path", &allPaths).Error
		if err != nil {
			return nil, err
		}
	}

	// Extract first segment after parentPath
	folderSet := make(map[string]bool)
	for _, path := range allPaths {
		var remaining string
		if parentPath == "" {
			remaining = path
		} else {
			// Remove "parentPath/" prefix
			remaining = strings.TrimPrefix(path, parentPath+"/")
		}

		// Get first segment
		if remaining != "" {
			parts := strings.SplitN(remaining, "/", 2)
			if parts[0] != "" {
				folderSet[parts[0]] = true
			}
		}
	}

	// Convert map to slice
	folders := make([]string, 0, len(folderSet))
	for folder := range folderSet {
		folders = append(folders, folder)
	}

	return folders, nil
}

func (r *ObjectRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&entity.Object{}, "id = ?", id).Error
}

func (r *ObjectRepository) DeleteByBucketID(bucketID uuid.UUID) error {
	return r.db.Delete(&entity.Object{}, "bucket_id = ?", bucketID).Error
}

func (r *ObjectRepository) FindByBucketIDAndHash(bucketID uuid.UUID, fileHash string) ([]entity.Object, error) {
	var objects []entity.Object
	err := r.db.Where("bucket_id = ? AND file_hash = ?", bucketID, fileHash).Find(&objects).Error
	if err != nil {
		return nil, err
	}
	return objects, nil
}
