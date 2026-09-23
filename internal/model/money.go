package model

import (
	"errors"
	"strconv"
	"strings"
)

// Money — сумма в минимальных единицах валюты (копейки, центы).
// У всех поддерживаемых валют (RUB, USD, EUR, GBP, CNY) две цифры после запятой.
type Money int64

// maxMoneyIntDigits — прежний предел NUMERIC(15,2): 13 цифр до запятой.
const maxMoneyIntDigits = 13

var ErrInvalidMoney = errors.New("invalid money amount")

// ParseMoney разбирает сумму из пользовательского ввода: "1 234,5", "1234.56", "-10".
// Пробелы (в т.ч. неразрывные) — разделители тысяч; дробная часть — не больше
// двух цифр, лишние знаки не округляются, а считаются ошибкой.
func ParseMoney(s string) (Money, error) {
	s = strings.Map(func(r rune) rune {
		switch r {
		case ' ', ' ', ' ':
			return -1
		case ',':
			return '.'
		}
		return r
	}, s)

	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, fracPart, _ := strings.Cut(s, ".")
	if intPart == "" && fracPart == "" {
		return 0, ErrInvalidMoney
	}
	if len(fracPart) > 2 || len(intPart) > maxMoneyIntDigits {
		return 0, ErrInvalidMoney
	}
	if !isDigits(intPart) || !isDigits(fracPart) {
		return 0, ErrInvalidMoney
	}

	for len(fracPart) < 2 {
		fracPart += "0"
	}
	minor, err := strconv.ParseInt(intPart+fracPart, 10, 64)
	if err != nil {
		return 0, ErrInvalidMoney
	}
	if neg {
		minor = -minor
	}
	return Money(minor), nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Decimal возвращает сумму в виде "-1234.56": для value в input и JS.
func (m Money) Decimal() string {
	sign := ""
	minor := int64(m)
	if minor < 0 {
		sign = "-"
		minor = -minor
	}
	frac := strconv.FormatInt(minor%100, 10)
	if len(frac) < 2 {
		frac = "0" + frac
	}
	return sign + strconv.FormatInt(minor/100, 10) + "." + frac
}

// MarshalJSON отдаёт сумму числом в основных единицах: графики в шаблонах
// получают 1234.56, а не 123456.
func (m Money) MarshalJSON() ([]byte, error) {
	return []byte(m.Decimal()), nil
}
