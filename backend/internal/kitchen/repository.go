package kitchen

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("tiket dapur tidak ditemukan")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// List mengambil tiket dapur beserta itemnya, urut RUSH lebih dahulu.
func (r *Repository) List(ctx context.Context, outletID string, f ListFilter) ([]Order, error) {
	const q = `
		SELECT ko.id, ko.order_id, o.order_no, o.queue_no, o.service_type,
		       t.number, ko.round_no, ko.status, ko.priority, ko.received_at,
		       ko.started_at, ko.ready_at, ko.served_at, ko.version
		FROM kitchen_orders ko
		JOIN orders o ON o.id = ko.order_id
		LEFT JOIN tables t ON t.id = o.table_id
		WHERE ko.outlet_id = $1
		  AND ko.status = ANY($2::text[])
		  AND ($3 = '' OR EXISTS (SELECT 1 FROM kitchen_order_items i
		                          WHERE i.kitchen_order_id = ko.id AND i.station = $3))
		ORDER BY (ko.priority = 'RUSH') DESC, ko.received_at`

	rows, err := r.pool.Query(ctx, q, outletID, f.Statuses, f.Station)
	if err != nil {
		return nil, fmt.Errorf("kitchenList: %w", err)
	}
	defer rows.Close()

	orders := []Order{}
	ids := []string{}
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.OrderID, &o.OrderNo, &o.QueueNo, &o.ServiceType,
			&o.TableName, &o.RoundNo, &o.Status, &o.Priority, &o.ReceivedAt,
			&o.StartedAt, &o.ReadyAt, &o.ServedAt, &o.Version); err != nil {
			return nil, fmt.Errorf("kitchenList: scan: %w", err)
		}
		o.Items = []Item{}
		orders = append(orders, o)
		ids = append(ids, o.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return orders, nil
	}

	byOrder := map[string][]Item{}
	irows, err := r.pool.Query(ctx, `
		SELECT kitchen_order_id, id, order_item_id, name_snapshot, variant_snapshot,
		       modifiers_text, note, quantity, station, status,
		       started_at, ready_at, served_at, version
		FROM kitchen_order_items
		WHERE kitchen_order_id = ANY($1::uuid[])
		ORDER BY created_at`, ids)
	if err != nil {
		return nil, fmt.Errorf("kitchenList: items: %w", err)
	}
	defer irows.Close()
	for irows.Next() {
		var kitchenID string
		var it Item
		if err := irows.Scan(&kitchenID, &it.ID, &it.OrderItemID, &it.Name, &it.Variant,
			&it.ModifiersText, &it.Note, &it.Quantity, &it.Station, &it.Status,
			&it.StartedAt, &it.ReadyAt, &it.ServedAt, &it.Version); err != nil {
			return nil, fmt.Errorf("kitchenList: scan item: %w", err)
		}
		byOrder[kitchenID] = append(byOrder[kitchenID], it)
	}
	if err := irows.Err(); err != nil {
		return nil, err
	}
	for i := range orders {
		if list, ok := byOrder[orders[i].ID]; ok {
			orders[i].Items = list
		}
	}
	return orders, nil
}

// History mengambil tiket yang sudah selesai atau void, dengan rentang waktu.
func (r *Repository) History(ctx context.Context, outletID string, f HistoryFilter) ([]Order, int, error) {
	const q = `
		SELECT ko.id, ko.order_id, o.order_no, o.queue_no, o.service_type,
		       t.number, ko.round_no, ko.status, ko.priority, ko.received_at,
		       ko.started_at, ko.ready_at, ko.served_at, ko.version,
		       count(*) OVER ()::int
		FROM kitchen_orders ko
		JOIN orders o ON o.id = ko.order_id
		LEFT JOIN tables t ON t.id = o.table_id
		WHERE ko.outlet_id = $1 AND ko.status IN ('SERVED', 'VOID')
		  AND ($2::timestamptz IS NULL OR ko.received_at >= $2)
		  AND ($3::timestamptz IS NULL OR ko.received_at < $3)
		ORDER BY ko.received_at DESC
		LIMIT $4 OFFSET $5`

	rows, err := r.pool.Query(ctx, q, outletID, f.From, f.To, f.PerPage, (f.Page-1)*f.PerPage)
	if err != nil {
		return nil, 0, fmt.Errorf("kitchenHistory: %w", err)
	}
	defer rows.Close()

	orders := []Order{}
	total := 0
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.OrderID, &o.OrderNo, &o.QueueNo, &o.ServiceType,
			&o.TableName, &o.RoundNo, &o.Status, &o.Priority, &o.ReceivedAt,
			&o.StartedAt, &o.ReadyAt, &o.ServedAt, &o.Version, &total); err != nil {
			return nil, 0, fmt.Errorf("kitchenHistory: scan: %w", err)
		}
		o.Items = []Item{}
		orders = append(orders, o)
	}
	return orders, total, rows.Err()
}

// lockedItem menyertakan konteks tiket dapur yang dipakai service untuk otorisasi.
type lockedItem struct {
	Item
	kitchenOrderID string
	outletID       string
}

// ItemForUpdate mengambil item dapur dan mengunci barisnya.
func (r *Repository) ItemForUpdate(ctx context.Context, tx pgx.Tx, id string) (*lockedItem, bool, error) {
	const q = `
		SELECT i.id, i.order_item_id, i.name_snapshot, i.variant_snapshot, i.modifiers_text,
		       i.note, i.quantity, i.station, i.status, i.started_at, i.ready_at,
		       i.served_at, i.version, ko.id, ko.outlet_id
		FROM kitchen_order_items i
		JOIN kitchen_orders ko ON ko.id = i.kitchen_order_id
		WHERE i.id = $1
		FOR UPDATE OF i`

	it := &lockedItem{}
	err := tx.QueryRow(ctx, q, id).Scan(&it.ID, &it.OrderItemID, &it.Name, &it.Variant,
		&it.ModifiersText, &it.Note, &it.Quantity, &it.Station, &it.Status, &it.StartedAt,
		&it.ReadyAt, &it.ServedAt, &it.Version, &it.kitchenOrderID, &it.outletID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("kitchenItemForUpdate: %w", err)
	}
	return it, true, nil
}

// Get mengambil satu tiket dapur beserta itemnya.
func (r *Repository) Get(ctx context.Context, outletID, id string) (*Order, error) {
	const q = `
		SELECT ko.id, ko.order_id, o.order_no, o.queue_no, o.service_type,
		       t.number, ko.round_no, ko.status, ko.priority, ko.received_at,
		       ko.started_at, ko.ready_at, ko.served_at, ko.version
		FROM kitchen_orders ko
		JOIN orders o ON o.id = ko.order_id
		LEFT JOIN tables t ON t.id = o.table_id
		WHERE ko.id = $1 AND ko.outlet_id = $2`

	o := &Order{}
	err := r.pool.QueryRow(ctx, q, id, outletID).Scan(&o.ID, &o.OrderID, &o.OrderNo, &o.QueueNo,
		&o.ServiceType, &o.TableName, &o.RoundNo, &o.Status, &o.Priority, &o.ReceivedAt,
		&o.StartedAt, &o.ReadyAt, &o.ServedAt, &o.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("kitchenGet: %w", err)
	}
	o.Items = []Item{}

	rows, err := r.pool.Query(ctx, `
		SELECT id, order_item_id, name_snapshot, variant_snapshot, modifiers_text,
		       note, quantity, station, status, started_at, ready_at, served_at, version
		FROM kitchen_order_items WHERE kitchen_order_id = $1 ORDER BY created_at`, o.ID)
	if err != nil {
		return nil, fmt.Errorf("kitchenGet: items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.OrderItemID, &it.Name, &it.Variant, &it.ModifiersText,
			&it.Note, &it.Quantity, &it.Station, &it.Status, &it.StartedAt, &it.ReadyAt,
			&it.ServedAt, &it.Version); err != nil {
			return nil, fmt.Errorf("kitchenGet: scan item: %w", err)
		}
		o.Items = append(o.Items, it)
	}
	return o, rows.Err()
}

// Begin membuka transaksi.
func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

// SetItemStatus menyimpan status item dengan optimistic lock pada version.
func (r *Repository) SetItemStatus(ctx context.Context, tx pgx.Tx, id, status string, version int) (bool, error) {
	const q = `
		UPDATE kitchen_order_items
		SET status = $2, version = version + 1,
		    started_at = CASE WHEN $2 = 'COOKING' AND started_at IS NULL THEN now() ELSE started_at END,
		    ready_at   = CASE WHEN $2 = 'READY'   AND ready_at   IS NULL THEN now() ELSE ready_at   END,
		    served_at  = CASE WHEN $2 = 'SERVED'  AND served_at  IS NULL THEN now() ELSE served_at  END
		WHERE id = $1 AND version = $3
		RETURNING id`

	var got string
	err := tx.QueryRow(ctx, q, id, status, version).Scan(&got)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("setKitchenItemStatus: %w", err)
	}
	return true, nil
}

// RecomputeOrderStatus menyimpulkan status tiket dapur dari status itemnya.
func (r *Repository) RecomputeOrderStatus(ctx context.Context, tx pgx.Tx, kitchenOrderID string) error {
	const q = `
		UPDATE kitchen_orders ko
		SET status = sub.st,
		    version = ko.version + 1,
		    started_at = COALESCE(ko.started_at, sub.started_at),
		    ready_at   = COALESCE(ko.ready_at, sub.ready_at),
		    served_at  = COALESCE(ko.served_at, sub.served_at)
		FROM (
			SELECT
				CASE
					WHEN count(*) FILTER (WHERE status <> 'VOID') = 0 THEN 'VOID'
					WHEN count(*) FILTER (WHERE status = 'SERVED') = count(*) FILTER (WHERE status <> 'VOID') THEN 'SERVED'
					WHEN count(*) FILTER (WHERE status IN ('READY', 'SERVED')) = count(*) FILTER (WHERE status <> 'VOID') THEN 'READY'
					WHEN count(*) FILTER (WHERE status IN ('COOKING', 'READY', 'SERVED')) > 0 THEN 'COOKING'
					ELSE 'NEW'
				END AS st,
				min(started_at) AS started_at,
				min(ready_at)   AS ready_at,
				min(served_at)  AS served_at
			FROM kitchen_order_items WHERE kitchen_order_id = $1
		) sub
		WHERE ko.id = $1`

	if _, err := tx.Exec(ctx, q, kitchenOrderID); err != nil {
		return fmt.Errorf("recomputeKitchenStatus: %w", err)
	}
	return nil
}

// SetPriority mengubah prioritas tiket dapur sekaligus pesanan induknya; false
// bila tiket tidak ada.
func (r *Repository) SetPriority(ctx context.Context, outletID, id, priority string) (bool, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	const q = `
		UPDATE kitchen_orders ko
		SET priority = $3, version = ko.version + 1
		FROM orders o
		WHERE o.id = ko.order_id AND ko.id = $1 AND ko.outlet_id = $2
		RETURNING o.id`
	var orderID string
	err = tx.QueryRow(ctx, q, id, outletID, priority).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("setKitchenPriority: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`UPDATE orders SET priority = $2, version = version + 1, updated_at = now() WHERE id = $1`,
		orderID, priority); err != nil {
		return false, fmt.Errorf("setKitchenPriority: order: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
