package repository

import (
	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-service/entity"
	"gorm.io/gorm"
)

type BucketRepository struct {
	db *gorm.DB
}

func NewBucketRepository(db *gorm.DB) *BucketRepository {
	return &BucketRepository{db: db}
}

func (r *BucketRepository) Create(bucket *entity.Bucket) error {
	return r.db.Create(bucket).Error
}

func (r *BucketRepository) FindByID(id uuid.UUID) (*entity.Bucket, error) {
	var bucket entity.Bucket
	err := r.db.Where("id = ?", id).First(&bucket).Error
	if err != nil {
		return nil, err
	}
	return &bucket, nil
}

func (r *BucketRepository) FindByName(name string) (*entity.Bucket, error) {
	var bucket entity.Bucket
	err := r.db.Where("name = ?", name).First(&bucket).Error
	if err != nil {
		return nil, err
	}
	return &bucket, nil
}

func (r *BucketRepository) ExistsByName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.Bucket{}).Where("name = ?", name).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *BucketRepository) FindByOwnerID(ownerID uuid.UUID) ([]entity.Bucket, error) {
	var buckets []entity.Bucket
	err := r.db.Where("owner_id = ?", ownerID).Find(&buckets).Error
	if err != nil {
		return nil, err
	}
	return buckets, nil
}

func (r *BucketRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&entity.Bucket{}, "id = ?", id).Error
}
