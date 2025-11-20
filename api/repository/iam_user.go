package repository

import (
	"errors"

	"github.com/tnqbao/gau-cloud-orchestrator/entity"
	"gorm.io/gorm"
)

// IAMUserRepository handles all database operations for IAM User entity
type IAMUserRepository struct {
	db *gorm.DB
}

func NewIAMUserRepository(db *gorm.DB) *IAMUserRepository {
	return &IAMUserRepository{
		db: db,
	}
}

func (r *IAMUserRepository) Create(user *entity.IAMUser) error {
	if user == nil {
		return errors.New("iam user cannot be nil")
	}
	return r.db.Create(user).Error
}

func (r *IAMUserRepository) GetByID(id uint) (*entity.IAMUser, error) {
	var user entity.IAMUser
	err := r.db.First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("iam user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *IAMUserRepository) GetByAccessKey(accessKey string) (*entity.IAMUser, error) {
	var user entity.IAMUser
	err := r.db.Where("access_key = ?", accessKey).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("iam user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *IAMUserRepository) GetByEmail(email string) (*entity.IAMUser, error) {
	var user entity.IAMUser
	err := r.db.Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("iam user not found")
		}
		return nil, err
	}
	return &user, nil
}

func (r *IAMUserRepository) Update(user *entity.IAMUser) error {
	if user == nil {
		return errors.New("iam user cannot be nil")
	}
	return r.db.Save(user).Error
}

func (r *IAMUserRepository) Delete(id uint) error {
	return r.db.Delete(&entity.IAMUser{}, id).Error
}

func (r *IAMUserRepository) List() ([]*entity.IAMUser, error) {
	var users []*entity.IAMUser
	err := r.db.Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *IAMUserRepository) GetByRole(role string) ([]*entity.IAMUser, error) {
	var users []*entity.IAMUser
	err := r.db.Where("role = ?", role).Find(&users).Error
	if err != nil {
		return nil, err
	}
	return users, nil
}

func (r *IAMUserRepository) ExistsByEmail(email string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.IAMUser{}).Where("email = ?", email).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *IAMUserRepository) ExistsByAccessKey(accessKey string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.IAMUser{}).Where("access_key = ?", accessKey).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *IAMUserRepository) CountByRole(role string) (int64, error) {
	var count int64
	err := r.db.Model(&entity.IAMUser{}).Where("role = ?", role).Count(&count).Error
	return count, err
}

func (r *IAMUserRepository) CheckIAMExistsByName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.IAMUser{}).Where("name = ?", name).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
