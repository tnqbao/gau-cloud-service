package repository

import (
	"errors"

	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-service/entity"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type IAMPolicyRepository struct {
	db *gorm.DB
}

func NewIAMPolicyRepository(db *gorm.DB) *IAMPolicyRepository {
	return &IAMPolicyRepository{
		db: db,
	}
}

func (r *IAMPolicyRepository) Create(policy *entity.IAMPolicy) error {
	if policy == nil {
		return errors.New("iam policy cannot be nil")
	}
	return r.db.Create(policy).Error
}

func (r *IAMPolicyRepository) GetByID(id uuid.UUID) (*entity.IAMPolicy, error) {
	var policy entity.IAMPolicy
	err := r.db.First(&policy, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("iam policy not found")
		}
		return nil, err
	}
	return &policy, nil
}

func (r *IAMPolicyRepository) GetByIAMID(iamID uuid.UUID) ([]*entity.IAMPolicy, error) {
	var policies []*entity.IAMPolicy
	err := r.db.Where("iam_id = ?", iamID).Find(&policies).Error
	if err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *IAMPolicyRepository) GetByIAMIDAndType(iamID uuid.UUID, policyType string) (*entity.IAMPolicy, error) {
	var policy entity.IAMPolicy
	err := r.db.Where("iam_id = ? AND type = ?", iamID, policyType).First(&policy).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("iam policy not found")
		}
		return nil, err
	}
	return &policy, nil
}

func (r *IAMPolicyRepository) Update(policy *entity.IAMPolicy) error {
	if policy == nil {
		return errors.New("iam policy cannot be nil")
	}
	return r.db.Save(policy).Error
}

func (r *IAMPolicyRepository) UpdatePolicy(id uuid.UUID, policyJSON datatypes.JSON) error {
	return r.db.Model(&entity.IAMPolicy{}).Where("id = ?", id).Update("policy", policyJSON).Error
}

func (r *IAMPolicyRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&entity.IAMPolicy{}, "id = ?", id).Error
}

func (r *IAMPolicyRepository) DeleteByIAMID(iamID uuid.UUID) error {
	return r.db.Where("iam_id = ?", iamID).Delete(&entity.IAMPolicy{}).Error
}

func (r *IAMPolicyRepository) List() ([]*entity.IAMPolicy, error) {
	var policies []*entity.IAMPolicy
	err := r.db.Find(&policies).Error
	if err != nil {
		return nil, err
	}
	return policies, nil
}

func (r *IAMPolicyRepository) ExistsByIAMIDAndType(iamID uuid.UUID, policyType string) (bool, error) {
	var count int64
	err := r.db.Model(&entity.IAMPolicy{}).Where("iam_id = ? AND type = ?", iamID, policyType).Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
