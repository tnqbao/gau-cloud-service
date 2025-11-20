package dto

type CreateIAMRequestDTO struct {
	AccessKey string `json:"access_key" binding:"required,min=8,max=64"`
	SecretKey string `json:"secret_key" binding:"required,min=16,max=128"`
	Name      string `json:"name" binding:"required,min=3,max=255"`
	Email     string `json:"email" binding:"required,email,max=255"`
	Role      string `json:"role" binding:"required,oneof=admin user viewer"`
}
