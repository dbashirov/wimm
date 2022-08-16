package v1

import (
	"context"
	"wimm/internal/domain/entity"
)

const (
	users = "/users"
)

type UserUsecase interface {
	ListAllUsers(ctx context.Context) ([]entity.User, error)
}

type userHandler struct {
	userUsecase UserUsecase
}

func NewUserHander(userUsecase UserUsecase) *userHandler {
	return &userHandler{userUsecase: userUsecase}
}
