package test

import (
	"context"
	"log"
	modelCategory "wimm/internal/domain/category/model"
	category "wimm/internal/domain/category/storage"
	"wimm/internal/domain/types/model"
	modelUser "wimm/internal/domain/user/model"
	user "wimm/internal/domain/user/storage"
)

func AddTestData(ur user.Repository, cr category.Repository) {

	// Создаем пользователья
	u := modelUser.User{
		Username: "user3",
		Email:    "user3@mail.com",
		Password: "qweasd",
	}
	err := ur.Create(context.TODO(), u)
	if err != nil {
		log.Printf("User creation error: %s\n", err)
		return
	}

	// Создаем категорию
	c := modelCategory.Category{
		Title: "Тест 3",
		User:  u,
		Type:  model.TypeExpense,
	}
	err = cr.Create(context.TODO(), &c)
	if err != nil {
		log.Printf("Category creation error: %s\n", err)
	}
}
