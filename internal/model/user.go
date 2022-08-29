package model

import "golang.org/x/crypto/bcrypt"

type User struct {
	ID                int    `json:"id"`
	Username          string `json:"username"`
	Email             string `json:"email"`
	Password          string `json:"password"`
	EncryptedPassword string `json:"encrypted_password"`
}

type CreateUserDTO struct {
	Username       string `json:"username"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	RepeatPassword string `json:"repeat_password"`
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

func encryptString(s string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.MinCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
