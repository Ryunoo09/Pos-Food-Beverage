package purchase

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/auth"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/httpx"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/pkg/money"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

type orderLine struct {
	input     OrderItemInput
	qty       money.Decimal
	price     money.Decimal
	discount  money.Decimal
	baseQty   money.Decimal
	lineTotal money.Decimal
}

// Create membuat PO berstatus DRAFT beserta itemnya.
func (s *Service) Create(ctx context.Context, actor auth.Actor, in CreateInput) (*PurchaseOrder, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	if strings.TrimSpace(in.SupplierID) == "" {
		return nil, validation("supplier_id wajib diisi.")
	}
	ok, err := s.repo.SupplierActive(ctx, actor.BusinessID, in.SupplierID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "SUPPLIER_NOT_FOUND",
			"Supplier tidak ditemukan atau tidak aktif.")
	}

	lines, subtotal, err := s.buildLines(ctx, actor.BusinessID, in.Items)
	if err != nil {
		return nil, err
	}
	discount, err := money.Parse(orZero(in.DiscountAmount))
	if err != nil || discount.IsNegative() {
		return nil, validation("discount_amount tidak valid.")
	}
	if discount.GreaterThan(subtotal) {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "PO_DISCOUNT_TOO_LARGE",
			"Diskon tidak boleh melebihi subtotal.")
	}
	total := subtotal.Sub(discount)

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// ponytail: periode pakai tanggal server; pakai business_day_cutoff bila perlu.
	period := time.Now().Format("060102")
	seq, err := s.repo.NextSequence(ctx, tx, actor.OutletID, "PURCHASE_ORDER", period)
	if err != nil {
		return nil, err
	}
	code := fmt.Sprintf("PO-%s-%04d", period, seq)

	id, err := s.repo.CreatePO(ctx, tx, actor.OutletID, code, actor.UserID, in.SupplierID,
		trimNote(in.ExpectedDate), trimNote(in.Note), amtStr(discount), amtStr(subtotal), amtStr(total))
	if err != nil {
		return nil, err
	}
	for _, ln := range lines {
		if err := s.repo.InsertPOItem(ctx, tx, id, ln.input.IngredientID, ln.input.PurchaseUnitID,
			qtyStr(ln.qty), amtStr(ln.price), amtStr(ln.discount),
			amtStr(ln.lineTotal), qtyStr(ln.baseQty)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.PO(ctx, actor.OutletID, id)
}

func (s *Service) List(ctx context.Context, actor auth.Actor, f Filter) ([]PurchaseOrder, int, error) {
	if actor.OutletID == "" {
		return nil, 0, outletMissing()
	}
	return s.repo.ListPO(ctx, actor.OutletID, f)
}

func (s *Service) Get(ctx context.Context, actor auth.Actor, id string) (*PurchaseOrder, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	o, err := s.repo.PO(ctx, actor.OutletID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, notFound()
	}
	return o, err
}

// Update mengubah PO yang masih DRAFT, termasuk mengganti seluruh item.
func (s *Service) Update(ctx context.Context, actor auth.Actor, id string, in UpdateInput) (*PurchaseOrder, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, _, _, found, err := s.repo.LockPO(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if status != "DRAFT" {
		return nil, httpx.NewError(http.StatusConflict, "PO_NOT_EDITABLE",
			"PO hanya bisa diubah pada status DRAFT.")
	}

	current, err := s.repo.PO(ctx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	discount := current.DiscountAmount
	if in.DiscountAmount != nil {
		discount = *in.DiscountAmount
	}
	disc, err := money.Parse(orZero(discount))
	if err != nil || disc.IsNegative() {
		return nil, validation("discount_amount tidak valid.")
	}

	subtotal := money.Decimal{}
	if in.Items != nil {
		lines, sum, err := s.buildLines(ctx, actor.BusinessID, *in.Items)
		if err != nil {
			return nil, err
		}
		subtotal = sum
		if err := s.repo.DeletePOItems(ctx, tx, id); err != nil {
			return nil, err
		}
		for _, ln := range lines {
			if err := s.repo.InsertPOItem(ctx, tx, id, ln.input.IngredientID, ln.input.PurchaseUnitID,
				qtyStr(ln.qty), amtStr(ln.price), amtStr(ln.discount),
				amtStr(ln.lineTotal), qtyStr(ln.baseQty)); err != nil {
				return nil, err
			}
		}
	} else {
		subtotal, err = money.Parse(current.Subtotal)
		if err != nil {
			return nil, err
		}
	}
	if disc.GreaterThan(subtotal) {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "PO_DISCOUNT_TOO_LARGE",
			"Diskon tidak boleh melebihi subtotal.")
	}
	total := subtotal.Sub(disc)

	expected := current.ExpectedDate
	if in.ExpectedDate != nil {
		expected = trimNote(in.ExpectedDate)
	}
	note := current.Note
	if in.Note != nil {
		note = trimNote(in.Note)
	}
	if err := s.repo.UpdatePOHeader(ctx, tx, actor.OutletID, id, expected, note,
		amtStr(disc), amtStr(subtotal), amtStr(total)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.PO(ctx, actor.OutletID, id)
}

func (s *Service) Submit(ctx context.Context, actor auth.Actor, id string) (*PurchaseOrder, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, _, _, found, err := s.repo.LockPO(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if status != "DRAFT" {
		return nil, httpx.NewError(http.StatusConflict, "PO_NOT_SUBMITTABLE",
			"PO tidak bisa diajukan pada status saat ini.")
	}
	po, err := s.repo.PO(ctx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if len(po.Items) == 0 {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "PO_EMPTY",
			"PO harus memiliki minimal satu item.")
	}
	ok, err := s.repo.SetPOSubmitted(ctx, tx, actor.OutletID, id, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusConflict, "PO_NOT_SUBMITTABLE",
			"PO tidak bisa diajukan pada status saat ini.")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.PO(ctx, actor.OutletID, id)
}

func (s *Service) Approve(ctx context.Context, actor auth.Actor, id string) (*PurchaseOrder, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, submittedBy, totalStr, found, err := s.repo.LockPO(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if status != "SUBMITTED" {
		return nil, httpx.NewError(http.StatusConflict, "PO_NOT_APPROVABLE",
			"PO tidak bisa disetujui pada status saat ini.")
	}

	thresholdStr, allowSelf, err := s.repo.ApproveSettings(ctx, actor.OutletID)
	if err != nil {
		return nil, err
	}
	threshold, err := money.Parse(orZero(thresholdStr))
	if err != nil {
		return nil, err
	}
	total, err := money.Parse(totalStr)
	if err != nil {
		return nil, err
	}
	needsSeparate := total.GreaterThan(threshold)
	if needsSeparate && !allowSelf && submittedBy != nil && *submittedBy == actor.UserID {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "PO_SELF_APPROVAL",
			"Pengaju tidak boleh menyetujui PO miliknya sendiri.")
	}

	ok, err := s.repo.SetPOApproved(ctx, tx, actor.OutletID, id, actor.UserID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusConflict, "PO_NOT_APPROVABLE",
			"PO tidak bisa disetujui pada status saat ini.")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.PO(ctx, actor.OutletID, id)
}

func (s *Service) Cancel(ctx context.Context, actor auth.Actor, id string, in CancelInput) (*PurchaseOrder, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return nil, validation("reason wajib diisi.")
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, _, _, found, err := s.repo.LockPO(ctx, tx, actor.OutletID, id)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if status == "RECEIVED" || status == "CANCELLED" {
		return nil, httpx.NewError(http.StatusConflict, "PO_NOT_CANCELLABLE",
			"PO tidak bisa dibatalkan pada status saat ini.")
	}
	ok, err := s.repo.SetPOCancelled(ctx, tx, actor.OutletID, id, reason)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusConflict, "PO_NOT_CANCELLABLE",
			"PO tidak bisa dibatalkan pada status saat ini.")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.PO(ctx, actor.OutletID, id)
}

// ReceiveForPO mencatat penerimaan atas PO dan memposting stok dengan MWAC.
func (s *Service) ReceiveForPO(ctx context.Context, actor auth.Actor, poID, key string, in ReceivingInput) (*Receiving, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	if strings.TrimSpace(key) == "" {
		return nil, validation("Header Idempotency-Key wajib diisi.")
	}
	if existing, ok, err := s.repo.ReceivingByKey(ctx, key); err != nil {
		return nil, err
	} else if ok {
		return s.repo.Receiving(ctx, actor.OutletID, existing)
	}
	if len(in.Items) == 0 {
		return nil, validation("items wajib diisi.")
	}
	for _, it := range in.Items {
		if it.POItemID == nil || strings.TrimSpace(*it.POItemID) == "" {
			return nil, validation("po_item_id wajib pada penerimaan berbasis PO.")
		}
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	status, _, _, found, err := s.repo.LockPO(ctx, tx, actor.OutletID, poID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, notFound()
	}
	if status != "APPROVED" && status != "PARTIALLY_RECEIVED" {
		return nil, httpx.NewError(http.StatusConflict, "PO_NOT_RECEIVABLE",
			"PO hanya bisa diterima setelah disetujui.")
	}

	items, err := s.prepareReceiving(ctx, actor.BusinessID, in.Items)
	if err != nil {
		return nil, err
	}

	period := time.Now().Format("060102")
	seq, err := s.repo.NextSequence(ctx, tx, actor.OutletID, "PURCHASE_RECEIVING", period)
	if err != nil {
		return nil, err
	}
	code := fmt.Sprintf("RCV-%s-%04d", period, seq)

	receivingID, err := s.repo.CreateReceiving(ctx, tx, actor.OutletID, code, &poID, in.SupplierID,
		actor.UserID, key, trimNote(in.SupplierInvoiceNo), trimNote(in.Note))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, httpx.NewError(http.StatusConflict, "RECEIVING_ALREADY_POSTED",
				"Penerimaan dengan kunci ini sudah diposting.")
		}
		return nil, err
	}

	for _, it := range items {
		txErr := s.postReceivingItem(ctx, tx, actor, receivingID, it, true)
		if txErr != nil {
			return nil, txErr
		}
	}

	all, err := s.repo.POReceiptProgress(ctx, tx, poID)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SetPOReceivedStatus(ctx, tx, actor.OutletID, poID, all); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Receiving(ctx, actor.OutletID, receivingID)
}

// ReceiveDirect mencatat penerimaan tanpa PO.
func (s *Service) ReceiveDirect(ctx context.Context, actor auth.Actor, key string, in ReceivingInput) (*Receiving, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	if strings.TrimSpace(key) == "" {
		return nil, validation("Header Idempotency-Key wajib diisi.")
	}
	if existing, ok, err := s.repo.ReceivingByKey(ctx, key); err != nil {
		return nil, err
	} else if ok {
		return s.repo.Receiving(ctx, actor.OutletID, existing)
	}
	if strings.TrimSpace(in.SupplierID) == "" {
		return nil, validation("supplier_id wajib diisi.")
	}
	ok, err := s.repo.SupplierActive(ctx, actor.BusinessID, in.SupplierID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, httpx.NewError(http.StatusUnprocessableEntity, "SUPPLIER_NOT_FOUND",
			"Supplier tidak ditemukan atau tidak aktif.")
	}
	if len(in.Items) == 0 {
		return nil, validation("items wajib diisi.")
	}

	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	items, err := s.prepareReceiving(ctx, actor.BusinessID, in.Items)
	if err != nil {
		return nil, err
	}

	period := time.Now().Format("060102")
	seq, err := s.repo.NextSequence(ctx, tx, actor.OutletID, "PURCHASE_RECEIVING", period)
	if err != nil {
		return nil, err
	}
	code := fmt.Sprintf("RCV-%s-%04d", period, seq)

	receivingID, err := s.repo.CreateReceiving(ctx, tx, actor.OutletID, code, nil, in.SupplierID,
		actor.UserID, key, trimNote(in.SupplierInvoiceNo), trimNote(in.Note))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, httpx.NewError(http.StatusConflict, "RECEIVING_ALREADY_POSTED",
				"Penerimaan dengan kunci ini sudah diposting.")
		}
		return nil, err
	}
	for _, it := range items {
		if err := s.postReceivingItem(ctx, tx, actor, receivingID, it, false); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.repo.Receiving(ctx, actor.OutletID, receivingID)
}

func (s *Service) GetReceiving(ctx context.Context, actor auth.Actor, id string) (*Receiving, error) {
	if actor.OutletID == "" {
		return nil, outletMissing()
	}
	o, err := s.repo.Receiving(ctx, actor.OutletID, id)
	if errors.Is(err, ErrNotFound) {
		return nil, httpx.NewError(http.StatusNotFound, "RECEIVING_NOT_FOUND",
			"Penerimaan tidak ditemukan.")
	}
	return o, err
}

type receivingLine struct {
	input       ReceivingItemInput
	qty         money.Decimal
	baseQty     money.Decimal
	unitPrice   money.Decimal
	discount    money.Decimal
	netUnitCost money.Decimal
	lineTotal   money.Decimal
}

func (s *Service) buildLines(ctx context.Context, businessID string, in []OrderItemInput) ([]orderLine, money.Decimal, error) {
	if len(in) == 0 {
		return nil, money.Decimal{}, validation("items wajib diisi.")
	}
	seen := map[string]bool{}
	lines := make([]orderLine, 0, len(in))
	subtotal := money.Decimal{}
	for _, it := range in {
		ingredientID := strings.TrimSpace(it.IngredientID)
		if ingredientID == "" {
			return nil, money.Decimal{}, validation("ingredient_id wajib diisi.")
		}
		if seen[ingredientID] {
			return nil, money.Decimal{}, validation("Bahan tidak boleh duplikat dalam satu PO.")
		}
		seen[ingredientID] = true
		ok, err := s.repo.IngredientActive(ctx, businessID, ingredientID)
		if err != nil {
			return nil, money.Decimal{}, err
		}
		if !ok {
			return nil, money.Decimal{}, httpx.NewError(http.StatusUnprocessableEntity,
				"INGREDIENT_NOT_FOUND", "Ada bahan yang tidak ditemukan atau tidak aktif.")
		}
		qty, err := money.Parse(strings.TrimSpace(it.QuantityOrdered))
		if err != nil || !qty.IsPositive() {
			return nil, money.Decimal{}, validation("quantity_ordered harus lebih dari 0.")
		}
		price, err := money.Parse(orZero(it.UnitPrice))
		if err != nil || price.IsNegative() {
			return nil, money.Decimal{}, validation("unit_price tidak valid.")
		}
		disc, err := money.Parse(orZero(it.DiscountAmount))
		if err != nil || disc.IsNegative() {
			return nil, money.Decimal{}, validation("discount_amount tidak valid.")
		}
		factorStr, ok, err := s.repo.UnitToBase(ctx, ingredientID, it.PurchaseUnitID)
		if err != nil {
			return nil, money.Decimal{}, err
		}
		if !ok {
			return nil, money.Decimal{}, httpx.NewError(http.StatusUnprocessableEntity,
				"UNIT_NOT_FOUND", "Satuan beli tidak ditemukan.")
		}
		factor, err := money.Parse(factorStr)
		if err != nil {
			return nil, money.Decimal{}, err
		}
		lineTotal := qty.Mul(price).Sub(disc)
		if lineTotal.IsNegative() {
			return nil, money.Decimal{}, httpx.NewError(http.StatusUnprocessableEntity,
				"PO_DISCOUNT_TOO_LARGE", "Diskon item tidak boleh melebihi nilai baris.")
		}
		subtotal = subtotal.Add(lineTotal)
		lines = append(lines, orderLine{
			input:     it,
			qty:       qty,
			price:     price,
			discount:  disc,
			baseQty:   qty.Mul(factor),
			lineTotal: lineTotal,
		})
	}
	return lines, subtotal, nil
}

func (s *Service) prepareReceiving(ctx context.Context, businessID string, in []ReceivingItemInput) ([]receivingLine, error) {
	seen := map[string]bool{}
	lines := make([]receivingLine, 0, len(in))
	for _, it := range in {
		ingredientID := strings.TrimSpace(it.IngredientID)
		if ingredientID == "" {
			return nil, validation("ingredient_id wajib diisi.")
		}
		if seen[ingredientID] {
			return nil, validation("Bahan tidak boleh duplikat dalam satu penerimaan.")
		}
		seen[ingredientID] = true
		ok, err := s.repo.IngredientActive(ctx, businessID, ingredientID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "INGREDIENT_NOT_FOUND",
				"Ada bahan yang tidak ditemukan atau tidak aktif.")
		}
		qty, err := money.Parse(strings.TrimSpace(it.QuantityReceived))
		if err != nil || !qty.IsPositive() {
			return nil, validation("quantity_received harus lebih dari 0.")
		}
		price, err := money.Parse(orZero(it.UnitPrice))
		if err != nil || price.IsNegative() {
			return nil, validation("unit_price tidak valid.")
		}
		disc, err := money.Parse(orZero(it.DiscountAllocated))
		if err != nil || disc.IsNegative() {
			return nil, validation("discount_allocated tidak valid.")
		}
		factorStr, found, err := s.repo.UnitToBase(ctx, ingredientID, it.PurchaseUnitID)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "UNIT_NOT_FOUND",
				"Satuan beli tidak ditemukan.")
		}
		factor, err := money.Parse(factorStr)
		if err != nil {
			return nil, err
		}
		baseQty := qty.Mul(factor)
		lineTotal := qty.Mul(price).Sub(disc)
		if lineTotal.IsNegative() {
			return nil, httpx.NewError(http.StatusUnprocessableEntity, "PO_DISCOUNT_TOO_LARGE",
				"Diskon tidak boleh melebihi nilai baris.")
		}
		netUnitCost := lineTotal.Div(baseQty).Round(6)
		lines = append(lines, receivingLine{
			input:       it,
			qty:         qty,
			baseQty:     baseQty,
			unitPrice:   price,
			discount:    disc,
			netUnitCost: netUnitCost,
			lineTotal:   lineTotal,
		})
	}
	// Kunci inventory selalu urut ingredient_id agar tidak deadlock.
	sort.Slice(lines, func(i, j int) bool {
		return lines[i].input.IngredientID < lines[j].input.IngredientID
	})
	return lines, nil
}

func (s *Service) postReceivingItem(ctx context.Context, tx pgx.Tx, actor auth.Actor, receivingID string,
	it receivingLine, withPO bool) error {
	ingredientID := strings.TrimSpace(it.input.IngredientID)

	if withPO && it.input.POItemID != nil {
		poIngredient, baseOrderedStr, receivedStr, found, err := s.repo.LockPOItem(ctx, tx, *it.input.POItemID)
		if err != nil {
			return err
		}
		if !found {
			return httpx.NewError(http.StatusUnprocessableEntity, "PO_ITEM_NOT_FOUND",
				"Baris PO tidak ditemukan.")
		}
		if poIngredient != ingredientID {
			return httpx.NewError(http.StatusUnprocessableEntity, "PO_ITEM_MISMATCH",
				"Bahan tidak sesuai dengan baris PO.")
		}
		baseOrdered, err := money.Parse(baseOrderedStr)
		if err != nil {
			return err
		}
		received, err := money.Parse(receivedStr)
		if err != nil {
			return err
		}
		if received.Add(it.baseQty).GreaterThan(baseOrdered) {
			return httpx.NewError(http.StatusUnprocessableEntity, "RECEIVING_EXCEEDS_PO",
				"Jumlah diterima melebihi sisa pesanan.")
		}
	}

	qtyRaw, avgRaw, err := s.repo.LockInventory(ctx, tx, actor.OutletID, ingredientID)
	if err != nil {
		return err
	}
	oldQty, err := money.Parse(qtyRaw)
	if err != nil {
		return err
	}
	oldAvg, err := money.Parse(avgRaw)
	if err != nil {
		return err
	}
	balance := oldQty.Add(it.baseQty)
	newAvg := mwac(oldQty, oldAvg, it.baseQty, it.netUnitCost)
	totalCost := money.RoundHalfUp(it.baseQty.Mul(it.netUnitCost), 2)

	movementID, err := s.repo.InsertPurchaseMovement(ctx, tx, actor.OutletID, ingredientID, receivingID,
		qtyStr(it.baseQty), costStr(it.netUnitCost), amtStr(totalCost), qtyStr(balance),
		costStr(newAvg), actor.UserID)
	if err != nil {
		if isUniqueViolation(err) {
			return httpx.NewError(http.StatusConflict, "RECEIVING_ALREADY_POSTED",
				"Penerimaan ini sudah diposting.")
		}
		return err
	}
	if err := s.repo.UpdateInventoryMWAC(ctx, tx, actor.OutletID, ingredientID,
		qtyStr(balance), costStr(newAvg), costStr(it.netUnitCost)); err != nil {
		return err
	}
	if err := s.repo.InsertCostHistory(ctx, tx, actor.OutletID, ingredientID, receivingID,
		costStr(it.netUnitCost), costStr(newAvg), qtyStr(it.baseQty)); err != nil {
		return err
	}
	if err := s.repo.InsertReceivingItem(ctx, tx, receivingID, it.input.POItemID, ingredientID,
		it.input.PurchaseUnitID, qtyStr(it.qty), qtyStr(it.baseQty),
		amtStr(it.unitPrice), amtStr(it.discount), costStr(it.netUnitCost), amtStr(it.lineTotal),
		movementID); err != nil {
		return err
	}
	if withPO && it.input.POItemID != nil {
		if err := s.repo.AddReceived(ctx, tx, *it.input.POItemID, qtyStr(it.baseQty)); err != nil {
			return err
		}
	}
	return nil
}

func validation(msg string) error {
	return httpx.NewError(http.StatusBadRequest, "VALIDATION_ERROR", msg)
}

func notFound() error {
	return httpx.NewError(http.StatusNotFound, "PO_NOT_FOUND", "Purchase order tidak ditemukan.")
}

func outletMissing() error {
	return httpx.NewError(http.StatusUnprocessableEntity, "OUTLET_NOT_CONFIGURED",
		"Outlet belum dikonfigurasi.")
}

func trimNote(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func orZero(s string) string {
	if strings.TrimSpace(s) == "" {
		return "0"
	}
	return s
}

func amtStr(d money.Decimal) string {
	return money.Format(d)
}

func qtyStr(d money.Decimal) string {
	return d.Round(4).StringFixed(4)
}

func costStr(d money.Decimal) string {
	return d.Round(6).StringFixed(6)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
