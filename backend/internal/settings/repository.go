package settings

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("pengaturan tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Settings(ctx context.Context, outletID string) (*OutletSettings, error) {
	if _, err := r.pool.Exec(ctx,
		`INSERT INTO outlet_settings (outlet_id) VALUES ($1) ON CONFLICT DO NOTHING`, outletID); err != nil {
		return nil, fmt.Errorf("settings: init: %w", err)
	}

	const q = `
		SELECT tax_inclusive, tax_on_service, rounding_mode,
		       discount_cashier_max_percent::text, discount_approval_threshold_amount::text,
		       refund_approval_threshold::text
		FROM outlet_settings WHERE outlet_id = $1`

	s := &OutletSettings{}
	err := r.pool.QueryRow(ctx, q, outletID).Scan(&s.TaxInclusive, &s.TaxOnService,
		&s.RoundingMode, &s.DiscountCashierMaxPercent, &s.DiscountApprovalThreshold,
		&s.RefundApprovalThreshold)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	return s, nil
}

func (r *Repository) UpdateSettings(ctx context.Context, outletID string, in OutletSettingsInput) (*OutletSettings, error) {
	const q = `
		UPDATE outlet_settings
		SET tax_inclusive = COALESCE($2, tax_inclusive),
		    tax_on_service = COALESCE($3, tax_on_service),
		    rounding_mode = COALESCE($4, rounding_mode),
		    discount_cashier_max_percent = COALESCE($5, discount_cashier_max_percent),
		    discount_approval_threshold_amount = COALESCE($6, discount_approval_threshold_amount),
		    refund_approval_threshold = COALESCE($7, refund_approval_threshold),
		    updated_at = now()
		WHERE outlet_id = $1`

	tag, err := r.pool.Exec(ctx, q, outletID, in.TaxInclusive, in.TaxOnService,
		in.RoundingMode, in.DiscountCashierMaxPercent, in.DiscountApprovalThreshold,
		in.RefundApprovalThreshold)
	if err != nil {
		return nil, fmt.Errorf("updateSettings: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return r.Settings(ctx, outletID)
}

func (r *Repository) Taxes(ctx context.Context, outletID string) ([]TaxConfig, error) {
	const q = `
		SELECT id, type, name, rate_percent::text, COALESCE(applies_to_service_types, '{}'),
		       is_active, effective_from
		FROM tax_configs
		WHERE outlet_id = $1
		ORDER BY is_active DESC, effective_from DESC, type`

	rows, err := r.pool.Query(ctx, q, outletID)
	if err != nil {
		return nil, fmt.Errorf("taxes: %w", err)
	}
	defer rows.Close()

	items := []TaxConfig{}
	for rows.Next() {
		var t TaxConfig
		if err := rows.Scan(&t.ID, &t.Type, &t.Name, &t.RatePercent, &t.AppliesTo,
			&t.IsActive, &t.EffectiveFrom); err != nil {
			return nil, fmt.Errorf("scan tax: %w", err)
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// CreateTax menonaktifkan konfigurasi jenis yang sama lalu menyimpan baris baru,
// sehingga riwayat tarif tetap tersimpan.
func (r *Repository) CreateTax(ctx context.Context, outletID string, in TaxConfigInput) (*TaxConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("createTax: begin: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`UPDATE tax_configs SET is_active = false WHERE outlet_id = $1 AND type = $2 AND is_active`,
		outletID, in.Type); err != nil {
		return nil, fmt.Errorf("createTax: deactivate: %w", err)
	}

	const q = `
		INSERT INTO tax_configs (outlet_id, type, name, rate_percent, applies_to_service_types, effective_from)
		VALUES ($1, $2, $3, $4, $5, COALESCE($6::date, current_date))
		RETURNING id, type, name, rate_percent::text, COALESCE(applies_to_service_types, '{}'),
		          is_active, effective_from`

	t := &TaxConfig{}
	if err := tx.QueryRow(ctx, q, outletID, in.Type, in.Name, in.RatePercent,
		in.AppliesTo, in.EffectiveFrom).Scan(&t.ID, &t.Type, &t.Name, &t.RatePercent,
		&t.AppliesTo, &t.IsActive, &t.EffectiveFrom); err != nil {
		return nil, fmt.Errorf("createTax: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("createTax: commit: %w", err)
	}
	return t, nil
}

func (r *Repository) SetTaxActive(ctx context.Context, outletID, id string, active bool) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE tax_configs SET is_active = $3 WHERE id = $1 AND outlet_id = $2`,
		id, outletID, active)
	if err != nil {
		return fmt.Errorf("setTaxActive: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
