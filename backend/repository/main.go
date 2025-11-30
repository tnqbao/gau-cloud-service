package repository

import (
	"github.com/tnqbao/gau-cloud-orchestrator/infra"
	"gorm.io/gorm"
)

type Repository struct {
	IAMUserRepo   *IAMUserRepository
	IAMPolicyRepo *IAMPolicyRepository
	BucketRepo    *BucketRepository
}

var repository *Repository

func InitRepository(infra *infra.Infra) *Repository {
	repository = &Repository{
		IAMUserRepo:   NewIAMUserRepository(infra.Postgres.DB),
		IAMPolicyRepo: NewIAMPolicyRepository(infra.Postgres.DB),
		BucketRepo:    NewBucketRepository(infra.Postgres.DB),
	}
	return repository
}

func GetRepository() *Repository {
	if repository == nil {
		panic("repository not initialized")
	}
	return repository
}

func (r *Repository) BeginTransaction(db *gorm.DB) *gorm.DB {
	return db.Begin()
}

func (r *Repository) WithTransaction(tx *gorm.DB) *Repository {
	return &Repository{
		IAMUserRepo:   NewIAMUserRepository(tx),
		IAMPolicyRepo: NewIAMPolicyRepository(tx),
		BucketRepo:    NewBucketRepository(tx),
	}
}
