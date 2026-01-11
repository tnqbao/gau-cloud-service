package repository

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tnqbao/gau-cloud-service/entity"
	"gorm.io/gorm"
)

type UploadSessionRepository struct {
	db *gorm.DB
}

func NewUploadSessionRepository(db *gorm.DB) *UploadSessionRepository {
	return &UploadSessionRepository{db: db}
}

// Create creates a new upload session
func (r *UploadSessionRepository) Create(session *entity.UploadSession) error {
	return r.db.Create(session).Error
}

// FindByID finds an upload session by its ID
func (r *UploadSessionRepository) FindByID(id uuid.UUID) (*entity.UploadSession, error) {
	var session entity.UploadSession
	err := r.db.Where("id = ?", id).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// FindByIDAndBucketID finds an upload session by ID and bucket ID
func (r *UploadSessionRepository) FindByIDAndBucketID(id, bucketID uuid.UUID) (*entity.UploadSession, error) {
	var session entity.UploadSession
	err := r.db.Where("id = ? AND bucket_id = ?", id, bucketID).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// FindByUserID finds all upload sessions for a user
func (r *UploadSessionRepository) FindByUserID(userID uuid.UUID) ([]entity.UploadSession, error) {
	var sessions []entity.UploadSession
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&sessions).Error
	return sessions, err
}

// FindActiveByBucketID finds active upload sessions for a bucket
func (r *UploadSessionRepository) FindActiveByBucketID(bucketID uuid.UUID) ([]entity.UploadSession, error) {
	var sessions []entity.UploadSession
	err := r.db.Where("bucket_id = ? AND status IN ?", bucketID,
		[]entity.UploadStatus{entity.UploadStatusInit, entity.UploadStatusUploading}).
		Order("created_at DESC").Find(&sessions).Error
	return sessions, err
}

// UpdateStatus updates the status of an upload session
func (r *UploadSessionRepository) UpdateStatus(id uuid.UUID, status entity.UploadStatus) error {
	return r.db.Model(&entity.UploadSession{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

// IncrementUploadedChunks increments the uploaded chunks count
func (r *UploadSessionRepository) IncrementUploadedChunks(id uuid.UUID) error {
	return r.db.Model(&entity.UploadSession{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"uploaded_chunks": gorm.Expr("uploaded_chunks + 1"),
			"status":          entity.UploadStatusUploading,
			"updated_at":      time.Now(),
		}).Error
}

// UpdateFileHash updates the file hash after composition
func (r *UploadSessionRepository) UpdateFileHash(id uuid.UUID, fileHash string) error {
	return r.db.Model(&entity.UploadSession{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"file_hash":  fileHash,
			"updated_at": time.Now(),
		}).Error
}

// Delete deletes an upload session
func (r *UploadSessionRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&entity.UploadSession{}, "id = ?", id).Error
}

// DeleteExpired deletes all expired upload sessions
func (r *UploadSessionRepository) DeleteExpired() (int64, error) {
	result := r.db.Where("expires_at < ? AND status NOT IN ?", time.Now(),
		[]entity.UploadStatus{entity.UploadStatusCompleted, entity.UploadStatusProcessing}).
		Delete(&entity.UploadSession{})
	return result.RowsAffected, result.Error
}

// FindExpired finds all expired upload sessions
func (r *UploadSessionRepository) FindExpired() ([]entity.UploadSession, error) {
	var sessions []entity.UploadSession
	err := r.db.Where("expires_at < ? AND status NOT IN ?", time.Now(),
		[]entity.UploadStatus{entity.UploadStatusCompleted, entity.UploadStatusProcessing}).
		Find(&sessions).Error
	return sessions, err
}

// GetUploadProgress returns the upload progress for a session
func (r *UploadSessionRepository) GetUploadProgress(id uuid.UUID) (*entity.UploadSession, error) {
	var session entity.UploadSession
	err := r.db.Select("id", "total_chunks", "uploaded_chunks", "status").
		Where("id = ?", id).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// ValidateAndLockSession validates session is active and locks for update
func (r *UploadSessionRepository) ValidateAndLockSession(id, bucketID uuid.UUID) (*entity.UploadSession, error) {
	var session entity.UploadSession
	err := r.db.Set("gorm:query_option", "FOR UPDATE").
		Where("id = ? AND bucket_id = ?", id, bucketID).
		First(&session).Error
	if err != nil {
		return nil, err
	}

	// Check if session is still active
	if session.Status != entity.UploadStatusInit && session.Status != entity.UploadStatusUploading {
		return nil, fmt.Errorf("upload session is not active, current status: %s", session.Status)
	}

	// Check if session has expired
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("upload session has expired")
	}

	return &session, nil
}
