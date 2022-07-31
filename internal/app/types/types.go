package types

type (
	TypeOfWallet    int
	TypeOfOperation int
)

const (
	TypeIncome TypeOfWallet = iota + 1
	TypeExpense
	TypeTransfer
)

func (t TypeOfWallet) String() string {
	return [...]string{"Доход", "Расход"}[t-1]
}

func (t TypeOfWallet) EnumIndex() int {
	return int(t)
}

func (t TypeOfOperation) String() string {
	return [...]string{"Доход", "Расход", "Перевод"}[t-1]
}

func (t TypeOfOperation) EnumIndex() int {
	return int(t)
}
