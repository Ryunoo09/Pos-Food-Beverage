package menu

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
)

const maxNameLength = 80

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) ListCategories(ctx context.Context, businessID string) ([]Category, error) {
	return s.repo.ListCategories(ctx, businessID)
}

func (s *Service) Category(ctx context.Context, businessID, id string) (*Category, error) {
	c, err := s.repo.Category(ctx, businessID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	return c, err
}

func (s *Service) CreateCategory(ctx context.Context, businessID string, in CategoryInput) (*Category, error) {
	in, err := validateCategory(in)
	if err != nil {
		return nil, err
	}

	c, err := s.repo.CreateCategory(ctx, businessID, in)
	if err != nil {
		return nil, mapCategoryError(err)
	}
	return c, nil
}

func (s *Service) UpdateCategory(ctx context.Context, businessID, id string, in CategoryInput) (*Category, error) {
	in, err := validateCategory(in)
	if err != nil {
		return nil, err
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	c, err := s.repo.UpdateCategory(ctx, businessID, id, in, isActive)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	if err != nil {
		return nil, mapCategoryError(err)
	}
	return c, nil
}

func (s *Service) DeleteCategory(ctx context.Context, businessID, id string) error {
	err := s.repo.DeleteCategory(ctx, businessID, id)
	if errors.Is(err, ErrNotFound) {
		return notFound()
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return httpx.NewError(http.StatusUnprocessableEntity, "CATEGORY_IN_USE",
				"Kategori masih dipakai produk. Nonaktifkan kategori atau pindahkan produknya.")
		}
		return err
	}
	return nil
}

func validateCategory(in CategoryInput) (CategoryInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > maxNameLength {
		return in, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Nama kategori wajib diisi, maksimal 80 karakter.")
	}
	return in, nil
}

func mapCategoryError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return httpx.NewError(http.StatusConflict, "CATEGORY_NAME_EXISTS",
			"Nama kategori sudah dipakai.")
	}
	return err
}

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "CATEGORY_NOT_FOUND", "Kategori tidak ditemukan.")
}
