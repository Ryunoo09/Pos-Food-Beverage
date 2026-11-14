// Package order mengelola pesanan: draft, item, dan (tahap berikut) konfirmasi.
package order

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pos"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, actor auth.Actor, f Filter) ([]Order, int, error) {
	if actor.OutletID == "" {
		return nil, 0, outletMissing()
	}
	if f.Status != nil && !validStatus(*f.Status) {
		return nil, 0, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "status tidak valid.")
	}
	return s.repo.List(ctx, actor.OutletID, f)
}

func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (*Order, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	o, err := s.repo.Order(ctx, actor.OutletID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	return o, err
}

func (s *Service) CreateDraft(ctx context.Context, actor auth.Actor, in DraftInput) (*Order, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}

	in.ServiceType = strings.TrimSpace(in.ServiceType)
	if !validServiceType(in.ServiceType) {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"service_type harus DINE_IN, TAKEAWAY, PICKUP, atau DELIVERY.")
	}
	in.Priority = strings.TrimSpace(in.Priority)
	if in.Priority == "" {
		in.Priority = "NORMAL"
	}
	if in.Priority != "NORMAL" && in.Priority != "RUSH" {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"priority harus NORMAL atau RUSH.")
	}

	if in.TableID != nil && *in.TableID != "" {
		owned, err := s.repo.TableOwned(ctx, actor.OutletID, *in.TableID)
		if err != nil {
			return nil, err
		}
		if !owned {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "TABLE_NOT_FOUND",
				"Meja tidak ditemukan atau tidak aktif.")
		}
	} else {
		in.TableID = nil
	}

	pricing, err := s.repo.Pricing(ctx, actor.OutletID, in.ServiceType)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, outletMissing()
		}
		return nil, err
	}

	o, err := s.repo.CreateDraft(ctx, actor.OutletID, actor.UserID, in, pricing)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, httpx.NewError(http.StatusConflict, "TABLE_OCCUPIED",
				"Meja sudah dipakai pesanan aktif lain.")
		}
		return nil, err
	}
	return o, nil
}

// Confirm menjalankan konfirmasi dengan proteksi idempotensi: permintaan ulang
// dengan kunci sama mengembalikan hasil yang sama.
func (s *Service) Confirm(ctx context.Context, actor auth.Actor, orderID, key string) (*Order, error) {
	if strings.TrimSpace(key) == "" {
		return nil, httpx.NewError(http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED",
			"Header Idempotency-Key wajib diisi.")
	}
	scope := actor.OutletID + ":" + actor.UserID + ":POST /orders/:id/confirm"
	hash := sha256.Sum256([]byte(orderID))

	claimed, err := s.repo.ClaimIdempotency(ctx, scope, key, hash[:])
	if err != nil {
		return nil, err
	}
	if !claimed {
		stored, status, body, found, err := s.repo.IdempotencyEntry(ctx, scope, key)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_CONFLICT",
				"Kunci idempotensi bertabrakan.")
		}
		if !bytes.Equal(stored, hash[:]) {
			return nil, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_CONFLICT",
				"Kunci idempotensi sudah dipakai untuk permintaan berbeda.")
		}
		if status == 0 {
			return nil, httpx.NewError(http.StatusConflict, "IDEMPOTENCY_IN_PROGRESS",
				"Permintaan dengan kunci ini sedang diproses.")
		}
		var o Order
		if err := json.Unmarshal(body, &o); err != nil {
			return nil, err
		}
		return &o, nil
	}

	o, err := s.confirm(ctx, actor, orderID)
	if err != nil {
		_ = s.repo.ReleaseIdempotency(ctx, scope, key)
		return nil, err
	}
	body, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveIdempotencyResponse(ctx, scope, key, http.StatusOK, body); err != nil {
		return nil, err
	}
	return o, nil
}

// confirm mengunci draft menjadi pesanan resmi: status, nomor order/antrean, dan
// snapshot harga final disimpan atomik.
func (s *Service) confirm(ctx context.Context, actor auth.Actor, orderID string) (*Order, error) {
	o, err := s.draft(ctx, actor, orderID)
	if err != nil {
		return nil, err
	}

	active := 0
	for _, item := range o.Items {
		if item.Status == "ACTIVE" {
			active++
		}
	}
	if active == 0 {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "ORDER_EMPTY",
			"Pesanan belum memiliki item.")
	}

	switch o.ServiceType {
	case "DINE_IN":
		if o.TableID == nil {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "TABLE_REQUIRED",
				"Pesanan dine-in wajib memilih meja.")
		}
	case "DELIVERY":
		if blank(o.CustomerPhone) || blank(o.DelivAddress) {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "DELIVERY_INFO_REQUIRED",
				"Pesanan delivery wajib nomor telepon dan alamat.")
		}
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Kurangi stok bahan dan hitung HPP dalam transaksi yang sama (BR-TX-1).
	lines, unknown, err := s.repo.StockLines(ctx, tx, o.ID)
	if err != nil {
		return nil, err
	}
	allowNegative, err := s.repo.AllowNegativeStock(ctx, tx, actor.OutletID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ApplyStock(ctx, tx, actor.OutletID, o.ID, actor.UserID, lines, allowNegative); err != nil {
		return nil, err
	}
	if err := s.repo.MarkCostUnknown(ctx, tx, unknown); err != nil {
		return nil, err
	}
	if err := s.repo.CreateKitchenOrder(ctx, tx, actor.OutletID, o.ID, o.Priority); err != nil {
		return nil, err
	}

	// ponytail: periode dari waktu server; business_day_cutoff belum diterapkan.
	period := time.Now().Format("060102")
	n, err := s.repo.NextSequence(ctx, tx, actor.OutletID, "ORDER", period)
	if err != nil {
		return nil, err
	}

	var queueNo *int
	if o.ServiceType == "TAKEAWAY" || o.ServiceType == "PICKUP" {
		q, err := s.repo.NextSequence(ctx, tx, actor.OutletID, "QUEUE", period)
		if err != nil {
			return nil, err
		}
		queueNo = &q
	}

	snapshot, err := json.Marshal(confirmSnapshot{
		pricingSnapshot: o.pricing,
		TaxRate:         o.TaxRate,
		ServiceRate:     o.ServiceRate,
		TaxInclusive:    o.TaxInclusive,
		Subtotal:        o.SubtotalAmount,
		ItemDiscount:    o.ItemDiscount,
		OrderDiscount:   o.OrderDiscount,
		Service:         o.ServiceAmount,
		Tax:             o.TaxAmount,
		Rounding:        o.RoundingAmount,
		Total:           o.TotalAmount,
	})
	if err != nil {
		return nil, err
	}

	if err := s.repo.SetConfirmed(ctx, tx, o.ID, actor.UserID, orderNo(period, n), queueNo, snapshot); err != nil {
		return nil, err
	}
	if err := s.repo.InsertStatusHistory(ctx, tx, o.ID, "DRAFT", "CONFIRMED", actor.UserID); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Order(ctx, actor.OutletID, o.ID)
}

func (s *Service) AddItem(ctx context.Context, actor auth.Actor, orderID string, in ItemInput) (*Order, error) {
	o, err := s.draft(ctx, actor, orderID)
	if err != nil {
		return nil, err
	}
	if in.Quantity <= 0 {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "quantity harus lebih dari nol.")
	}

	sv, err := s.repo.SaleVariant(ctx, actor.BusinessID, in.ProductID, in.VariantID)
	if errors.Is(err, ErrNotFound) {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "PRODUCT_VARIANT_NOT_FOUND",
			"Produk atau varian tidak tersedia untuk dijual.")
	}
	if err != nil {
		return nil, err
	}

	var choices []modifierChoice
	if len(in.ModifierOptionIDs) > 0 {
		choices, err = s.repo.ModifierOptions(ctx, actor.BusinessID, in.ModifierOptionIDs)
		if err != nil {
			return nil, err
		}
		if len(choices) != len(unique(in.ModifierOptionIDs)) {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "MODIFIER_OPTION_INVALID",
				"Modifier tidak valid atau tidak aktif.")
		}
	}

	unitPrice, _ := money.Parse(sv.UnitPrice)
	modifiersTotal, _ := money.Parse("0")
	for _, c := range choices {
		delta, err := money.Parse(c.PriceDelta)
		if err != nil {
			return nil, err
		}
		modifiersTotal = modifiersTotal.Add(delta)
	}

	lineTotal := unitPrice.Add(modifiersTotal).Mul(money.FromInt(int64(in.Quantity)))
	if _, err := s.repo.AddItem(ctx, o.ID, sv, in, choices, money.Format(modifiersTotal),
		money.Format(lineTotal)); err != nil {
		return nil, err
	}
	return s.recalc(ctx, actor, o.ID)
}

func (s *Service) UpdateItem(ctx context.Context, actor auth.Actor, orderID, itemID string, in UpdateItemInput) (*Order, error) {
	o, err := s.draft(ctx, actor, orderID)
	if err != nil {
		return nil, err
	}
	if in.Quantity == nil && in.Note == nil {
		return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR",
			"Kirim quantity atau note yang ingin diubah.")
	}

	quantity := 0
	if in.Quantity != nil {
		if *in.Quantity <= 0 {
			return nil, httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", "quantity harus lebih dari nol.")
		}
		quantity = *in.Quantity
	}

	if in.Quantity != nil {
		ok, err := s.repo.UpdateItemQuantity(ctx, o.ID, itemID, quantity, in.Note)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, itemNotFound()
		}
	} else {
		ok, err := s.repo.UpdateItemNote(ctx, o.ID, itemID, in.Note)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, itemNotFound()
		}
	}
	return s.recalc(ctx, actor, o.ID)
}

func (s *Service) DeleteItem(ctx context.Context, actor auth.Actor, orderID, itemID string) (*Order, error) {
	o, err := s.draft(ctx, actor, orderID)
	if err != nil {
		return nil, err
	}
	ok, err := s.repo.DeleteItem(ctx, o.ID, itemID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, itemNotFound()
	}
	return s.recalc(ctx, actor, o.ID)
}

// draft memastikan pesanan ada, milik outlet, dan masih DRAFT sebelum diubah.
func (s *Service) draft(ctx context.Context, actor auth.Actor, orderID string) (*Order, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	o, err := s.repo.Order(ctx, actor.OutletID, orderID)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	if err != nil {
		return nil, err
	}
	if o.Status != "DRAFT" {
		return nil, httpx.NewError(http.StatusConflict, "ORDER_NOT_DRAFT",
			"Pesanan sudah dikonfirmasi dan tidak bisa diubah.")
	}
	return o, nil
}

// recalc menghitung ulang total otoritatif di server lalu menyimpannya.
func (s *Service) recalc(ctx context.Context, actor auth.Actor, orderID string) (*Order, error) {
	o, err := s.repo.Order(ctx, actor.OutletID, orderID)
	if err != nil {
		return nil, err
	}

	items := make([]pos.Item, 0, len(o.Items))
	for _, item := range o.Items {
		if item.Status != "ACTIVE" {
			continue
		}
		unitPrice, err := money.Parse(item.UnitPrice)
		if err != nil {
			return nil, err
		}
		modifiers, err := money.Parse(item.ModifiersTotal)
		if err != nil {
			return nil, err
		}
		items = append(items, pos.Item{
			UnitPrice: money.Format(unitPrice.Add(modifiers)),
			Quantity:  item.Quantity,
		})
	}

	totals := orderTotals{
		Subtotal: "0.00", ItemDiscount: "0.00", OrderDiscount: "0.00",
		Service: "0.00", Tax: "0.00", Rounding: "0.00", Total: "0.00",
	}
	if len(items) > 0 {
		result, err := pos.Calculate(pos.Input{
			Items:        items,
			TaxRate:      o.TaxRate,
			ServiceRate:  o.ServiceRate,
			TaxInclusive: o.TaxInclusive,
			TaxOnService: o.pricing.TaxOnService,
			RoundingMode: o.pricing.RoundingMode,
		})
		if err != nil {
			return nil, err
		}
		totals = orderTotals{
			Subtotal:      result.Subtotal,
			ItemDiscount:  result.ItemDiscount,
			OrderDiscount: result.OrderDiscount,
			Service:       result.Service,
			Tax:           result.Tax,
			Rounding:      result.RoundingAdjustment,
			Total:         result.Total,
		}
	}

	if err := s.repo.SaveTotals(ctx, o.ID, totals); err != nil {
		return nil, err
	}
	return s.repo.Order(ctx, actor.OutletID, o.ID)
}

func validServiceType(v string) bool {
	switch v {
	case "DINE_IN", "TAKEAWAY", "PICKUP", "DELIVERY":
		return true
	}
	return false
}

func blank(s *string) bool {
	return s == nil || strings.TrimSpace(*s) == ""
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// confirmSnapshot menyimpan setelan harga final saat pesanan dikonfirmasi.
type confirmSnapshot struct {
	pricingSnapshot
	TaxRate       string `json:"tax_rate"`
	ServiceRate   string `json:"service_rate"`
	TaxInclusive  bool   `json:"tax_inclusive"`
	Subtotal      string `json:"subtotal"`
	ItemDiscount  string `json:"item_discount"`
	OrderDiscount string `json:"order_discount"`
	Service       string `json:"service"`
	Tax           string `json:"tax"`
	Rounding      string `json:"rounding"`
	Total         string `json:"total"`
}

func validStatus(v string) bool {
	switch v {
	case "DRAFT", "CONFIRMED", "IN_PROGRESS", "READY", "SERVED", "COMPLETED", "CANCELLED":
		return true
	}
	return false
}

func unique(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "ORDER_NOT_FOUND", "Pesanan tidak ditemukan.")
}

func itemNotFound() error {
	return httpx.NewError(http.StatusNotFound, "ORDER_ITEM_NOT_FOUND", "Item pesanan tidak ditemukan.")
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}
