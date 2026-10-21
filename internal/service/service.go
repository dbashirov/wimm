package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/dbashirov/wimm/internal/model"
	"github.com/dbashirov/wimm/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidEmail       = errors.New("invalid email address")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrNotFound           = errors.New("not found")
	ErrWrongCategoryType  = errors.New("category type must match operation type")
	ErrSameWallet         = errors.New("source and target wallet must differ")
	ErrHasOperations      = errors.New("has linked operations")
	// ErrToAmountRequired — перевод между валютами без суммы зачисления.
	ErrToAmountRequired = errors.New("target amount required for cross-currency transfer")
)

// ValidationError - некорректный ввод пользователя; Msg показывается как есть.
type ValidationError struct {
	Msg string
}

func (e *ValidationError) Error() string { return e.Msg }

// maxReorderIDs - столько кошельков у пользователя не бывает, а без лимита
// тело в 1 MB разворачивается в десятки тысяч id одного запроса.
const maxReorderIDs = 1000

// normalizeEmail приводит email к каноническому виду (lowercase, trimmed).
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// dummyPasswordHash используется в Login для уравнивания времени работы
// bcrypt при отсутствии пользователя (защита от timing attack).
var dummyPasswordHash []byte

func init() {
	h, err := bcrypt.GenerateFromPassword([]byte("dummy-for-timing"), 12)
	if err != nil {
		panic("bcrypt init: " + err.Error())
	}
	dummyPasswordHash = h
}

type Service struct {
	users      storage.UserRepo
	wallets    storage.WalletRepo
	categories storage.CategoryRepo
	operations storage.OperationRepo
	store      *storage.Storage // для Ping в healthcheck; в тестах nil
}

// New создаёт Service из конкретного storage.Storage (продакшн-путь).
func New(store *storage.Storage) *Service {
	return &Service{
		users:      store.Users,
		wallets:    store.Wallets,
		categories: store.Categories,
		operations: store.Operations,
		store:      store,
	}
}

// Ping проверяет работоспособность БД для readiness-проб.
func (s *Service) Ping(ctx context.Context) error {
	if s.store == nil {
		return nil
	}
	return s.store.Ping(ctx)
}

// NewWithRepos создаёт Service из интерфейсов напрямую (используется в тестах).
func NewWithRepos(
	users storage.UserRepo,
	wallets storage.WalletRepo,
	categories storage.CategoryRepo,
	operations storage.OperationRepo,
) *Service {
	return &Service{
		users:      users,
		wallets:    wallets,
		categories: categories,
		operations: operations,
	}
}

// --- Users ---

// newUserSeed — стартовый набор нового пользователя: без кошелька и категорий
// после регистрации пустой экран и не добавить ни одной операции.
func newUserSeed() model.UserSeed {
	expense := []string{
		"Продукты", "Кафе и рестораны", "Транспорт", "Жильё и ЖКХ",
		"Связь и интернет", "Здоровье", "Одежда", "Развлечения",
		"Подарки", "Прочие расходы",
	}
	income := []string{"Зарплата", "Подработка", "Проценты и кешбэк", "Прочие доходы"}

	seed := model.UserSeed{
		Wallet:     model.WalletInput{Name: "Основной", Currency: "RUB"},
		Categories: make([]model.CategoryInput, 0, len(expense)+len(income)),
	}
	for _, name := range expense {
		seed.Categories = append(seed.Categories,
			model.CategoryInput{Name: name, Type: model.TypeExpense})
	}
	for _, name := range income {
		seed.Categories = append(seed.Categories,
			model.CategoryInput{Name: name, Type: model.TypeIncome})
	}
	return seed
}

func (s *Service) Register(ctx context.Context, email, password string) (*model.User, error) {
	email = normalizeEmail(email)
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, ErrInvalidEmail
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, fmt.Errorf("bcrypt: %w", err)
	}
	user, err := s.users.Create(ctx, email, string(hash), newUserSeed())
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return user, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (*model.User, error) {
	email = normalizeEmail(email)
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		// Сравниваем пароль с фиктивным хешем чтобы выровнять время ответа
		// и не давать злоумышленнику возможность перечислять зарегистрированных
		// пользователей по разнице во времени (timing attack).
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash), []byte(password),
	); err != nil {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

// --- Wallets ---

func (s *Service) ListWallets(ctx context.Context, userID uuid.UUID) ([]model.Wallet, error) {
	return s.wallets.ListByUser(ctx, userID)
}

func (s *Service) ListWalletsWithBalance(ctx context.Context, userID uuid.UUID) ([]model.WalletBalance, error) {
	return s.wallets.ListWithBalance(ctx, userID)
}

func (s *Service) CreateWallet(ctx context.Context, userID uuid.UUID, inp model.WalletInput) (*model.Wallet, error) {
	return s.wallets.Create(ctx, userID, inp)
}

func (s *Service) GetWallet(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Wallet, error) {
	w, err := s.wallets.GetByID(ctx, id, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return w, err
}

func (s *Service) UpdateWallet(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
	inp model.WalletInput,
) (*model.Wallet, error) {
	w, err := s.wallets.Update(ctx, id, userID, inp)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return w, err
}

func (s *Service) DeleteWallet(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	err := s.wallets.Delete(ctx, id, userID)
	return wrapDeleteErr(err)
}

func (s *Service) ReorderWallets(
	ctx context.Context,
	userID uuid.UUID,
	ids []uuid.UUID,
) error {
	if len(ids) == 0 || len(ids) > maxReorderIDs {
		return &ValidationError{Msg: "некорректный список кошельков"}
	}
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return &ValidationError{Msg: "кошелёк указан в списке дважды"}
		}
		seen[id] = struct{}{}
	}
	return s.wallets.Reorder(ctx, userID, ids)
}

func (s *Service) SetDefaultWallet(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) (*model.Wallet, error) {
	w, err := s.wallets.SetDefault(ctx, id, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return w, err
}

func (s *Service) ToggleWalletHidden(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) (*model.Wallet, error) {
	w, err := s.wallets.ToggleHidden(ctx, id, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return w, err
}

// --- Categories ---

func (s *Service) ListCategories(ctx context.Context, userID uuid.UUID) ([]model.Category, error) {
	return s.categories.ListByUser(ctx, userID)
}

func (s *Service) ListCategoriesByType(
	ctx context.Context,
	userID uuid.UUID,
	catType model.OperationType,
) ([]model.Category, error) {
	return s.categories.ListByType(ctx, userID, catType)
}

func (s *Service) CreateCategory(
	ctx context.Context,
	userID uuid.UUID,
	inp model.CategoryInput,
) (*model.Category, error) {
	return s.categories.Create(ctx, userID, inp)
}

func (s *Service) GetCategory(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Category, error) {
	c, err := s.categories.GetByID(ctx, id, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return c, err
}

func (s *Service) UpdateCategory(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
	name, description string,
) (*model.Category, error) {
	c, err := s.categories.Update(ctx, id, userID, name, description)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return c, err
}

func (s *Service) DeleteCategory(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	err := s.categories.Delete(ctx, id, userID)
	return wrapDeleteErr(err)
}

// --- Operations ---

func (s *Service) ListOperations(ctx context.Context, filter model.OperationFilter) ([]model.Operation, error) {
	return s.operations.List(ctx, filter)
}

func (s *Service) CreateOperation(
	ctx context.Context,
	userID uuid.UUID,
	inp model.OperationInput,
) (*model.Operation, error) {
	if _, err := s.wallets.GetByID(ctx, inp.WalletID, userID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	cat, err := s.categories.GetByID(ctx, inp.CategoryID, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if cat.Type != inp.Type {
		return nil, ErrWrongCategoryType
	}
	return s.operations.Create(ctx, userID, inp)
}

func (s *Service) GetOperation(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*model.Operation, error) {
	o, err := s.operations.GetByID(ctx, id, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return o, err
}

func (s *Service) UpdateOperation(
	ctx context.Context,
	id uuid.UUID,
	userID uuid.UUID,
	inp model.OperationInput,
) (*model.Operation, error) {
	if _, err := s.wallets.GetByID(ctx, inp.WalletID, userID); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	cat, err := s.categories.GetByID(ctx, inp.CategoryID, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if cat.Type != inp.Type {
		return nil, ErrWrongCategoryType
	}
	o, err := s.operations.Update(ctx, id, userID, inp)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	return o, err
}

func (s *Service) DeleteOperation(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return wrapNotFound(s.operations.Delete(ctx, id, userID))
}

func (s *Service) CreateTransfer(
	ctx context.Context,
	userID uuid.UUID,
	inp model.TransferInput,
) (*model.Operation, *model.Operation, error) {
	inp, err := s.prepareTransfer(ctx, userID, inp)
	if err != nil {
		return nil, nil, err
	}
	return s.operations.CreateTransfer(ctx, userID, inp)
}

// prepareTransfer проверяет кошельки перевода и выставляет сумму зачисления:
// в одной валюте она всегда равна списанию, между валютами — обязательна.
func (s *Service) prepareTransfer(
	ctx context.Context,
	userID uuid.UUID,
	inp model.TransferInput,
) (model.TransferInput, error) {
	if inp.FromWalletID == inp.ToWalletID {
		return inp, ErrSameWallet
	}
	from, err := s.wallets.GetByID(ctx, inp.FromWalletID, userID)
	if err != nil {
		return inp, wrapNotFound(err)
	}
	to, err := s.wallets.GetByID(ctx, inp.ToWalletID, userID)
	if err != nil {
		return inp, wrapNotFound(err)
	}
	if from.Currency == to.Currency {
		inp.ToAmount = inp.Amount
		return inp, nil
	}
	if inp.ToAmount <= 0 {
		return inp, ErrToAmountRequired
	}
	return inp, nil
}

func (s *Service) GetTransfer(
	ctx context.Context,
	transferID, userID uuid.UUID,
) (*model.Operation, *model.Operation, error) {
	exp, inc, err := s.operations.GetTransfer(ctx, transferID, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, nil, ErrNotFound
	}
	return exp, inc, err
}

func (s *Service) UpdateTransfer(
	ctx context.Context,
	transferID, userID uuid.UUID,
	inp model.TransferInput,
) (*model.Operation, *model.Operation, error) {
	inp, err := s.prepareTransfer(ctx, userID, inp)
	if err != nil {
		return nil, nil, err
	}
	exp, inc, err := s.operations.UpdateTransfer(ctx, transferID, userID, inp)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, nil, ErrNotFound
	}
	return exp, inc, err
}

func (s *Service) DeleteTransfer(ctx context.Context, transferID uuid.UUID, userID uuid.UUID) error {
	return wrapNotFound(s.operations.DeleteTransfer(ctx, transferID, userID))
}

// --- Wallet stats ---

func (s *Service) WalletStats(
	ctx context.Context,
	walletID uuid.UUID,
	userID uuid.UUID,
) (*model.DashboardData, error) {
	wallet, err := s.wallets.GetByID(ctx, walletID, userID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	expenses, incomes, monthly, err := s.operations.WalletDashboardData(ctx, walletID, userID)
	if err != nil {
		return nil, err
	}
	var balance model.Money
	for _, mt := range monthly {
		balance = mt.Balance
	}
	return &model.DashboardData{
		WalletBalances: []model.WalletBalance{{
			Wallet:  *wallet,
			Balance: balance,
		}},
		TotalBalances: []model.CurrencyAmount{{
			Currency: wallet.Currency,
			Amount:   balance,
		}},
		MonthlyExpenses: expenses,
		MonthlyIncomes:  incomes,
		MonthlyTotals:   monthly,
	}, nil
}

// --- Dashboard ---

func (s *Service) DashboardData(ctx context.Context, userID uuid.UUID) (*model.DashboardData, error) {
	wallets, err := s.wallets.ListWithBalance(ctx, userID)
	if err != nil {
		return nil, err
	}
	expenses, incomes, monthly, err := s.operations.DashboardData(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &model.DashboardData{
		WalletBalances:  wallets,
		TotalBalances:   totalsByCurrency(wallets),
		MonthlyExpenses: expenses,
		MonthlyIncomes:  incomes,
		MonthlyTotals:   monthly,
	}, nil
}

// totalsByCurrency суммирует балансы кошельков отдельно по каждой валюте,
// в порядке первого появления валюты в списке кошельков.
func totalsByCurrency(wallets []model.WalletBalance) []model.CurrencyAmount {
	var totals []model.CurrencyAmount
	index := make(map[string]int)
	for _, w := range wallets {
		i, ok := index[w.Currency]
		if !ok {
			i = len(totals)
			index[w.Currency] = i
			totals = append(totals, model.CurrencyAmount{Currency: w.Currency})
		}
		totals[i].Amount += w.Balance
	}
	return totals
}

func wrapNotFound(err error) error {
	if errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

// wrapDeleteErr преобразует storage-ошибки при удалении в service-уровневые.
// pgcode 23503 — нарушение FK, значит на объект ссылаются операции.
func wrapDeleteErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, storage.ErrNotFound) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return ErrHasOperations
	}
	return err
}
