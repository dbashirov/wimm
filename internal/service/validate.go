package service

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/dbashirov/wimm/internal/model"
)

const (
	maxNameLen        = 100
	maxDescriptionLen = 500
	maxEmailLen       = 254
	minPasswordLen    = 8
	// bcrypt на пароле длиннее 72 байт возвращает ошибку
	maxPasswordBytes = 72
)

// validateName обрезает пробелы и проверяет название кошелька или категории.
func validateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", &ValidationError{Msg: "Укажите название"}
	}
	if utf8.RuneCountInString(name) > maxNameLen {
		return "", &ValidationError{
			Msg: fmt.Sprintf("Название длиннее %d символов", maxNameLen),
		}
	}
	return name, nil
}

func validateDescription(description string) error {
	if utf8.RuneCountInString(description) > maxDescriptionLen {
		return &ValidationError{
			Msg: fmt.Sprintf("Описание длиннее %d символов", maxDescriptionLen),
		}
	}
	return nil
}

// validateCurrency приводит код валюты к верхнему регистру и проверяет
// формат ISO 4217 (три латинские буквы): колонка CHAR(3) иначе отвечает 500.
func validateCurrency(currency string) (string, error) {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if len(currency) != 3 {
		return "", &ValidationError{Msg: "Некорректный код валюты"}
	}
	for _, c := range currency {
		if c < 'A' || c > 'Z' {
			return "", &ValidationError{Msg: "Некорректный код валюты"}
		}
	}
	return currency, nil
}

func validateType(t model.OperationType) error {
	if t != model.TypeIncome && t != model.TypeExpense {
		return &ValidationError{Msg: "Некорректный тип"}
	}
	return nil
}

func validatePassword(password string) error {
	if utf8.RuneCountInString(password) < minPasswordLen {
		return &ValidationError{
			Msg: fmt.Sprintf("Пароль должен быть не менее %d символов", minPasswordLen),
		}
	}
	if len(password) > maxPasswordBytes {
		return &ValidationError{Msg: "Пароль слишком длинный"}
	}
	return nil
}

func validateWallet(inp model.WalletInput) (model.WalletInput, error) {
	var err error
	if inp.Name, err = validateName(inp.Name); err != nil {
		return inp, err
	}
	if inp.Currency, err = validateCurrency(inp.Currency); err != nil {
		return inp, err
	}
	return inp, validateDescription(inp.Description)
}

func validateCategory(inp model.CategoryInput) (model.CategoryInput, error) {
	var err error
	if inp.Name, err = validateName(inp.Name); err != nil {
		return inp, err
	}
	if err := validateType(inp.Type); err != nil {
		return inp, err
	}
	return inp, validateDescription(inp.Description)
}
