package model

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                int    `json:"id"`
	Username          string `json:"username" validate:"required"`
	Email             string `json:"email" validate:"required,email"`
	Password          string `json:"password,omitempty" validate:"required"`
	EncryptedPassword string `json:"encrypted_password,omitempty" validate:"required"`
}

type CreateUserDTO struct {
	Username       string `json:"username"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	RepeatPassword string `json:"repeat_password"`
}

func (u *User) Validate() error {
	if u.Password != u.EncryptedPassword {
		return fmt.Errorf("password do not match")
	}
	return validator.New().Struct(u)
}

func (u *User) BeforeCreate() error {
	if len(u.Password) > 0 {
		enc, err := encryptString(u.Password)
		if err != nil {
			return err
		}
		u.EncryptedPassword = enc
	}
	return nil
}

func (u *User) ComparePawwword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.EncryptedPassword), []byte(password)) == nil
}

func (u *User) Cleaning() {
	u.Password = ""
	u.EncryptedPassword = ""
}

func encryptString(s string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.MinCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
