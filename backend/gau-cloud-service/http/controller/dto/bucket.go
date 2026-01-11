package dto

type CreateBucketRequestDTO struct {
	Name   string `json:"name" binding:"required,min=3,max=63"`
	Region string `json:"region" binding:"required"`
}

type UpdateBucketAccessRequestDTO struct {
	Access string `json:"access" binding:"required,oneof=public private"`
}
