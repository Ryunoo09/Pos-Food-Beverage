// Command seed-domain mengisi data demo yang bisa langsung dipakai Fase 5:
// kategori, menu + varian, modifier, bahan + stok awal, supplier, meja, dan
// metode bayar. Aman dijalankan berulang: baris dicari dulu lewat kunci unik.
//
// Jalankan dari folder backend/: go run ./cmd/seed-domain
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ryunoo09/pos-food-beverage/backend/internal/config"
	"github.com/ryunoo09/pos-food-beverage/backend/internal/db"
)

type variantSpec struct {
	name      string
	skuSuffix string
	price     string
	isDefault bool
}

type productSpec struct {
	category  string
	sku       string
	name      string
	desc      string
	kind      string
	stockMode string
	station   string
	variants  []variantSpec
	modifiers []string
}

type ingredientSpec struct {
	category string
	code     string
	name     string
	baseUnit string
	buyUnit  string
	minStock string
	onHand   string
	avgCost  string
}

type tableSpec struct {
	area     string
	number   string
	capacity int
	sort     int
}

var categories = []string{"Makanan", "Minuman", "Camilan", "Kopi", "Non-Kopi"}

var products = []productSpec{
	{"Makanan", "NGK-01", "Nasi Goreng Kampung", "Nasi goreng dengan telur, ayam, dan sambal terasi.", "FOOD", "TRACK_BY_RECIPE", "KITCHEN",
		[]variantSpec{{"Reguler", "REG", "28000.00", true}, {"Jumbo", "JMB", "36000.00", false}},
		[]string{"Level Pedas", "Tambahan"}},
	{"Makanan", "MIG-01", "Mie Goreng Spesial", "Mie goreng sayur dengan telur dan ayam suwir.", "FOOD", "TRACK_BY_RECIPE", "KITCHEN",
		[]variantSpec{{"Reguler", "REG", "26000.00", true}, {"Spesial", "SPC", "32000.00", false}},
		[]string{"Level Pedas", "Tambahan"}},
	{"Makanan", "AYB-01", "Ayam Bakar Madu", "Ayam bakar bumbu madu dengan lalapan.", "FOOD", "TRACK_BY_RECIPE", "KITCHEN",
		[]variantSpec{{"Reguler", "REG", "34000.00", true}, {"Paha Atas", "PHA", "39000.00", false}},
		[]string{"Level Pedas", "Tambahan"}},
	{"Makanan", "SAP-01", "Sate Ayam 10 Tusuk", "Sate ayam dengan bumbu kacang dan kecap.", "FOOD", "TRACK_BY_RECIPE", "KITCHEN",
		[]variantSpec{{"10 Tusuk", "10", "30000.00", true}, {"20 Tusuk", "20", "56000.00", false}},
		[]string{"Level Pedas"}},
	{"Makanan", "BKR-01", "Bakso Sapi Urat", "Bakso sapi dengan kuah kaldu dan mie kuning.", "FOOD", "TRACK_BY_RECIPE", "KITCHEN",
		[]variantSpec{{"Reguler", "REG", "24000.00", true}}, nil},
	{"Camilan", "TAH-01", "Tahu Crispy", "Tahu goreng crispy dengan saus kacang.", "FOOD", "TRACK_BY_RECIPE", "KITCHEN",
		[]variantSpec{{"Porsi Kecil", "S", "15000.00", true}, {"Porsi Besar", "L", "22000.00", false}},
		[]string{"Tambahan"}},
	{"Camilan", "PIS-01", "Pisang Goreng Keju", "Pisang goreng dengan topping keju dan susu.", "FOOD", "TRACK_BY_RECIPE", "KITCHEN",
		[]variantSpec{{"Reguler", "REG", "18000.00", true}}, []string{"Tambahan"}},
	{"Kopi", "ESP-01", "Es Kopi Susu", "Espresso, susu segar, dan gula aren.", "BEVERAGE", "TRACK_BY_RECIPE", "BAR",
		[]variantSpec{{"Reguler", "REG", "22000.00", true}, {"Large", "LRG", "27000.00", false}},
		[]string{"Suhu", "Level Gula"}},
	{"Kopi", "AME-01", "Americano", "Espresso dengan air dingin atau panas.", "BEVERAGE", "TRACK_BY_RECIPE", "BAR",
		[]variantSpec{{"Reguler", "REG", "18000.00", true}}, []string{"Suhu", "Level Gula"}},
	{"Kopi", "LAT-01", "Cafe Latte", "Espresso dengan susu steamed.", "BEVERAGE", "TRACK_BY_RECIPE", "BAR",
		[]variantSpec{{"Reguler", "REG", "25000.00", true}, {"Large", "LRG", "30000.00", false}},
		[]string{"Suhu", "Level Gula"}},
	{"Non-Kopi", "MAT-01", "Matcha Latte", "Matcha Jepang dengan susu segar.", "BEVERAGE", "TRACK_BY_RECIPE", "BAR",
		[]variantSpec{{"Reguler", "REG", "26000.00", true}, {"Large", "LRG", "31000.00", false}},
		[]string{"Suhu", "Level Gula"}},
	{"Minuman", "TEH-01", "Es Teh Manis", "Teh tubruk dengan gula batu.", "BEVERAGE", "TRACK_BY_RECIPE", "BAR",
		[]variantSpec{{"Reguler", "REG", "8000.00", true}, {"Jumbo", "JMB", "12000.00", false}},
		[]string{"Level Gula"}},
	{"Minuman", "LEM-01", "Es Lemon Tea", "Teh dengan perasan lemon segar.", "BEVERAGE", "TRACK_BY_RECIPE", "BAR",
		[]variantSpec{{"Reguler", "REG", "14000.00", true}}, []string{"Level Gula"}},
	{"Minuman", "AIR-01", "Air Mineral", "Botol 600 ml.", "BEVERAGE", "TRACK_AS_STOCK_ITEM", "BAR",
		[]variantSpec{{"Botol", "BTL", "5000.00", true}}, nil},
	{"Minuman", "BOB-01", "Boba Milk Tea", "Teh susu dengan boba kenyal.", "BEVERAGE", "TRACK_BY_RECIPE", "BAR",
		[]variantSpec{{"Reguler", "REG", "24000.00", true}, {"Large", "LRG", "29000.00", false}},
		[]string{"Suhu", "Level Gula", "Topping"}},
}

var modifierGroups = []struct {
	name      string
	minSelect int
	maxSelect int
	required  bool
	options   []struct {
		name, delta string
		isDefault   bool
	}
}{
	{"Level Pedas", 0, 1, false, []struct {
		name, delta string
		isDefault   bool
	}{
		{"Tidak pedas", "0", true}, {"Pedas sedang", "0", false}, {"Pedas maksimal", "0", false},
	}},
	{"Tambahan", 0, 3, false, []struct {
		name, delta string
		isDefault   bool
	}{
		{"Telur", "5000", false}, {"Keju", "6000", false}, {"Sambal extra", "2000", false},
	}},
	{"Suhu", 1, 1, true, []struct {
		name, delta string
		isDefault   bool
	}{
		{"Es", "0", true}, {"Panas", "0", false},
	}},
	{"Level Gula", 0, 1, false, []struct {
		name, delta string
		isDefault   bool
	}{
		{"Normal", "0", true}, {"Less sugar", "0", false}, {"Tanpa gula", "0", false},
	}},
	{"Topping", 0, 2, false, []struct {
		name, delta string
		isDefault   bool
	}{
		{"Boba extra", "4000", false}, {"Jelly", "4000", false}, {"Cream cheese", "7000", false},
	}},
}

var ingredients = []ingredientSpec{
	{"Karbohidrat", "BRS-01", "Beras pandan wangi", "kg", "kg", "10", "25", "14500"},
	{"Karbohidrat", "MIE-01", "Mie kuning", "kg", "kg", "5", "8", "18000"},
	{"Protein", "AYM-01", "Ayam fillet", "kg", "kg", "4", "9", "38000"},
	{"Protein", "DAG-01", "Daging sapi", "kg", "kg", "3", "6", "125000"},
	{"Protein", "TLR-01", "Telur ayam", "pcs", "pcs", "30", "80", "2400"},
	{"Protein", "TAH-01", "Tahu putih", "pcs", "pcs", "20", "40", "1500"},
	{"Bumbu", "BWG-01", "Bawang merah", "kg", "kg", "2", "4", "32000"},
	{"Bumbu", "BWP-01", "Bawang putih", "kg", "kg", "2", "3.5", "36000"},
	{"Bumbu", "CBR-01", "Cabai rawit merah", "kg", "kg", "1", "2", "48000"},
	{"Bumbu", "GUL-01", "Gula pasir", "kg", "kg", "5", "12", "17000"},
	{"Minyak", "MYK-01", "Minyak goreng", "l", "l", "8", "18", "19000"},
	{"Kopi", "KOP-01", "Biji kopi arabika", "kg", "kg", "3", "5", "185000"},
	{"Dairy", "SUS-01", "Susu UHT full cream", "l", "l", "6", "14", "21000"},
	{"Dairy", "KEJ-01", "Keju cheddar", "kg", "kg", "1", "2.5", "165000"},
	{"Minuman", "TEH-01", "Teh tubruk", "kg", "kg", "1", "2", "85000"},
	{"Minuman", "ESB-01", "Es batu", "kg", "kg", "10", "30", "2000"},
	{"Minuman", "BOB-01", "Boba kering", "kg", "kg", "2", "3", "45000"},
	{"Minuman", "AIR-01", "Air mineral botol 600 ml", "pcs", "pcs", "24", "72", "2800"},
}

var suppliers = []struct {
	name            string
	contact         string
	phone           string
	address         string
	ingredientCodes []string
}{
	{"CV Sumber Pangan", "Bu Rina", "0812-3456-7801", "Jl. Pasar Induk No. 12, Jakarta", []string{"BRS-01", "MIE-01", "GUL-01", "MYK-01"}},
	{"PT Protein Nusantara", "Pak Budi", "0813-2211-4455", "Jl. Raya Bogor KM 24, Jakarta", []string{"AYM-01", "DAG-01", "TLR-01"}},
	{"Sayur Segar Harian", "Kak Wati", "0857-9090-1122", "Pasar Minggu Blok C, Jakarta", []string{"BWG-01", "BWP-01", "CBR-01", "TAH-01"}},
	{"Roastery Kopi Kita", "Mas Dimas", "0821-7788-9900", "Jl. Kemang Selatan No. 8, Jakarta", []string{"KOP-01", "SUS-01", "KEJ-01"}},
	{"Bubble & Tea Supply", "Ci Lina", "0818-3344-5566", "Jl. Mangga Besar No. 45, Jakarta", []string{"TEH-01", "BOB-01", "ESB-01", "AIR-01"}},
}

var tables = []tableSpec{
	{"Indoor", "M1", 2, 1}, {"Indoor", "M2", 2, 2}, {"Indoor", "M3", 4, 3},
	{"Indoor", "M4", 4, 4}, {"Indoor", "M5", 6, 5}, {"Indoor", "M6", 6, 6},
	{"Teras", "T1", 4, 1}, {"Teras", "T2", 4, 2}, {"Teras", "T3", 2, 3}, {"Teras", "T4", 8, 4},
	{"Bar", "B1", 2, 1}, {"Bar", "B2", 2, 2},
}

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		fail("konfigurasi: %v", err)
	}
	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		fail("koneksi database: %v", err)
	}
	defer pool.Close()

	if err := seed(ctx, pool); err != nil {
		fail("seed domain: %v", err)
	}
	fmt.Printf("Seed domain selesai: %d kategori, %d menu, %d bahan, %d supplier, %d meja.\n",
		len(categories), len(products), len(ingredients), len(suppliers), len(tables))
}

func seed(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var businessID, outletID string
	if err := tx.QueryRow(ctx, `SELECT id FROM businesses ORDER BY created_at, id LIMIT 1`).Scan(&businessID); err != nil {
		return fmt.Errorf("bisnis: jalankan cmd/seed dulu: %w", err)
	}
	if err := tx.QueryRow(ctx,
		`SELECT id FROM outlets WHERE business_id = $1 ORDER BY is_default DESC, created_at LIMIT 1`,
		businessID).Scan(&outletID); err != nil {
		return fmt.Errorf("outlet: jalankan cmd/seed dulu: %w", err)
	}

	categoryIDs := map[string]string{}
	for index, name := range categories {
		var id string
		err := tx.QueryRow(ctx,
			`INSERT INTO categories (business_id, name, sort_order)
			 VALUES ($1, $2, $3)
			 ON CONFLICT (business_id, lower(name)) DO UPDATE SET is_active = true
			 RETURNING id`, businessID, name, index).Scan(&id)
		if err != nil {
			return fmt.Errorf("kategori %s: %w", name, err)
		}
		categoryIDs[name] = id
	}

	modifierIDs, err := seedModifiers(ctx, tx, businessID)
	if err != nil {
		return err
	}

	productIDs := map[string]string{}
	for index, product := range products {
		categoryID, ok := categoryIDs[product.category]
		if !ok {
			return fmt.Errorf("kategori menu %s tidak dikenal", product.category)
		}
		var id string
		err := tx.QueryRow(ctx,
			`INSERT INTO products (business_id, category_id, sku, name, description,
			                       product_type, stock_mode, kitchen_station, sort_order,
			                       is_active, is_available)
			 VALUES ($1, $2, $3, $4, $5, $6,
			         CASE WHEN $7 = 'TRACK_AS_STOCK_ITEM' THEN 'TRACK_BY_RECIPE' ELSE $7 END,
			         NULLIF($8, ''), $9, true, true)
			 ON CONFLICT (business_id, sku) DO UPDATE SET
			   category_id = EXCLUDED.category_id, name = EXCLUDED.name,
			   description = EXCLUDED.description, product_type = EXCLUDED.product_type,
			   stock_mode = EXCLUDED.stock_mode, kitchen_station = EXCLUDED.kitchen_station,
			   sort_order = EXCLUDED.sort_order, is_active = true, is_available = true,
			   updated_at = now()
			 RETURNING id`,
			businessID, categoryID, product.sku, product.name, product.desc,
			product.kind, product.stockMode, product.station, index).Scan(&id)
		if err != nil {
			return fmt.Errorf("menu %s: %w", product.sku, err)
		}
		productIDs[product.sku] = id

		for variantIndex, variant := range product.variants {
			if variant.isDefault {
				if _, err := tx.Exec(ctx,
					`UPDATE product_variants SET is_default = false
					 WHERE product_id = $1 AND is_default AND name <> $2`,
					id, variant.name); err != nil {
					return fmt.Errorf("reset varian default %s: %w", product.sku, err)
				}
			}
			_, err := tx.Exec(ctx,
				`INSERT INTO product_variants (product_id, name, sku_suffix, price, is_default, sort_order)
				 SELECT $1, $2, $3, $4, $5, $6
				 WHERE NOT EXISTS (
				   SELECT 1 FROM product_variants WHERE product_id = $1 AND name = $2)`,
				id, variant.name, variant.skuSuffix, variant.price, variant.isDefault, variantIndex)
			if err != nil {
				return fmt.Errorf("varian %s/%s: %w", product.sku, variant.name, err)
			}
			if variant.isDefault {
				if _, err := tx.Exec(ctx,
					`UPDATE product_variants SET is_default = true
					 WHERE product_id = $1 AND name = $2`, id, variant.name); err != nil {
					return fmt.Errorf("set varian default %s/%s: %w", product.sku, variant.name, err)
				}
			}
		}

		for groupIndex, groupName := range product.modifiers {
			groupID, ok := modifierIDs[groupName]
			if !ok {
				return fmt.Errorf("grup modifier %s tidak dikenal", groupName)
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO product_modifier_groups (product_id, modifier_group_id, sort_order)
				 VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`, id, groupID, groupIndex); err != nil {
				return fmt.Errorf("modifier %s/%s: %w", product.sku, groupName, err)
			}
		}
	}

	unitIDs := map[string]int16{}
	rows, err := tx.Query(ctx, `SELECT code, id FROM units`)
	if err != nil {
		return fmt.Errorf("satuan: %w", err)
	}
	for rows.Next() {
		var code string
		var id int16
		if err := rows.Scan(&code, &id); err != nil {
			rows.Close()
			return fmt.Errorf("scan satuan: %w", err)
		}
		unitIDs[code] = id
	}
	rows.Close()

	ingredientIDs := map[string]string{}
	for _, ingredient := range ingredients {
		categoryID, err := ensureIngredientCategory(ctx, tx, businessID, ingredient.category)
		if err != nil {
			return err
		}
		baseUnit, ok := unitIDs[ingredient.baseUnit]
		if !ok {
			return fmt.Errorf("satuan %s tidak dikenal", ingredient.baseUnit)
		}
		purchaseUnit, ok := unitIDs[ingredient.buyUnit]
		if !ok {
			return fmt.Errorf("satuan beli %s tidak dikenal", ingredient.buyUnit)
		}

		var id string
		err = tx.QueryRow(ctx,
			`INSERT INTO ingredients (business_id, code, name, category_id, base_unit_id,
			                          default_purchase_unit_id, min_stock, is_stock_product)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 ON CONFLICT (business_id, code) DO UPDATE SET
			   name = EXCLUDED.name, category_id = EXCLUDED.category_id,
			   base_unit_id = EXCLUDED.base_unit_id,
			   default_purchase_unit_id = EXCLUDED.default_purchase_unit_id,
			   min_stock = EXCLUDED.min_stock, is_active = true, updated_at = now()
			 RETURNING id`,
			businessID, ingredient.code, ingredient.name, categoryID, baseUnit,
			purchaseUnit, ingredient.minStock,
			ingredient.code == "AIR-01").Scan(&id)
		if err != nil {
			return fmt.Errorf("bahan %s: %w", ingredient.code, err)
		}
		ingredientIDs[ingredient.code] = id

		var hasStock bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM inventory WHERE outlet_id = $1 AND ingredient_id = $2)`,
			outletID, id).Scan(&hasStock); err != nil {
			return fmt.Errorf("cek stok %s: %w", ingredient.code, err)
		}
		if hasStock {
			continue
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO inventory (outlet_id, ingredient_id, quantity_on_hand, avg_unit_cost,
			                        last_purchase_cost, last_movement_at)
			 VALUES ($1, $2, $3, $4, $4, now())`,
			outletID, id, ingredient.onHand, ingredient.avgCost); err != nil {
			return fmt.Errorf("stok awal %s: %w", ingredient.code, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO stock_movements (outlet_id, ingredient_id, movement_type,
			                              quantity_delta, unit_cost, total_cost,
			                              balance_after, avg_cost_after, reference_type)
			 VALUES ($1, $2, 'INITIAL_STOCK', $3, $4, $5, $3, $4, 'SEED')`,
			outletID, id, ingredient.onHand, ingredient.avgCost,
			fmt.Sprintf("%.2f", mustFloat(ingredient.onHand)*mustFloat(ingredient.avgCost))); err != nil {
			return fmt.Errorf("movement awal %s: %w", ingredient.code, err)
		}
	}

	// Produk ber-stock item ditautkan setelah bahan dibuat (bahan di-seed sesudah
	// menu), karena check constraint mewajibkan stock_ingredient_id saat insert.
	for _, product := range products {
		if product.stockMode != "TRACK_AS_STOCK_ITEM" {
			continue
		}
		if _, err := tx.Exec(ctx,
			`UPDATE products SET stock_mode = 'TRACK_AS_STOCK_ITEM', stock_ingredient_id = i.id
			 FROM ingredients i
			 WHERE products.business_id = $1 AND i.business_id = $1
			   AND products.sku = $2 AND i.code = $2`,
			businessID, product.sku); err != nil {
			return fmt.Errorf("tautkan stok %s: %w", product.sku, err)
		}
	}

	for _, supplier := range suppliers {
		var id string
		err := tx.QueryRow(ctx,
			`INSERT INTO suppliers (business_id, name, contact_person, phone, address)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (business_id, lower(name)) DO UPDATE SET
			   contact_person = EXCLUDED.contact_person, phone = EXCLUDED.phone,
			   address = EXCLUDED.address, is_active = true, updated_at = now()
			 RETURNING id`,
			businessID, supplier.name, supplier.contact, supplier.phone, supplier.address).Scan(&id)
		if err != nil {
			return fmt.Errorf("supplier %s: %w", supplier.name, err)
		}
		for _, code := range supplier.ingredientCodes {
			ingredientID, ok := ingredientIDs[code]
			if !ok {
				return fmt.Errorf("bahan supplier %s tidak dikenal", code)
			}
			if _, err := tx.Exec(ctx,
				`INSERT INTO supplier_ingredients (supplier_id, ingredient_id, is_preferred)
				 VALUES ($1, $2, true) ON CONFLICT DO NOTHING`, id, ingredientID); err != nil {
				return fmt.Errorf("bahan supplier %s/%s: %w", supplier.name, code, err)
			}
		}
	}

	for _, table := range tables {
		if _, err := tx.Exec(ctx,
			`INSERT INTO tables (outlet_id, area, number, capacity, sort_order)
			 VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (outlet_id, number) DO UPDATE SET
			   area = EXCLUDED.area, capacity = EXCLUDED.capacity,
			   sort_order = EXCLUDED.sort_order, is_active = true`,
			outletID, table.area, table.number, table.capacity, table.sort); err != nil {
			return fmt.Errorf("meja %s: %w", table.number, err)
		}
	}

	paymentMethods := []struct {
		code, name string
		isCash     bool
		reference  bool
		sort       int
	}{
		{"CASH", "Tunai", true, false, 1},
		{"QRIS", "QRIS", false, true, 2},
		{"TRANSFER", "Transfer bank", false, true, 3},
		{"E_WALLET", "E-Wallet", false, true, 4},
	}
	for _, method := range paymentMethods {
		if _, err := tx.Exec(ctx,
			`INSERT INTO payment_methods (outlet_id, code, name, is_cash, requires_reference, sort_order)
			 VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (outlet_id, code) DO UPDATE SET
			   name = EXCLUDED.name, is_cash = EXCLUDED.is_cash,
			   requires_reference = EXCLUDED.requires_reference,
			   sort_order = EXCLUDED.sort_order, is_active = true`,
			outletID, method.code, method.name, method.isCash, method.reference, method.sort); err != nil {
			return fmt.Errorf("metode bayar %s: %w", method.code, err)
		}
	}

	return tx.Commit(ctx)
}

func seedModifiers(ctx context.Context, tx pgx.Tx, businessID string) (map[string]string, error) {
	ids := map[string]string{}
	for groupIndex, group := range modifierGroups {
		var id string
		err := tx.QueryRow(ctx,
			`SELECT id FROM modifier_groups WHERE business_id = $1 AND name = $2`,
			businessID, group.name).Scan(&id)
		if err == pgx.ErrNoRows {
			err = tx.QueryRow(ctx,
				`INSERT INTO modifier_groups (business_id, name, min_select, max_select, is_required, sort_order)
				 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
				businessID, group.name, group.minSelect, group.maxSelect, group.required, groupIndex).Scan(&id)
		} else if err == nil {
			_, err = tx.Exec(ctx,
				`UPDATE modifier_groups SET min_select = $3, max_select = $4,
				 is_required = $5, is_active = true, updated_at = now()
				 WHERE id = $1 AND business_id = $2`,
				id, businessID, group.minSelect, group.maxSelect, group.required)
		}
		if err != nil {
			return nil, fmt.Errorf("grup modifier %s: %w", group.name, err)
		}
		ids[group.name] = id

		for optionIndex, option := range group.options {
			if _, err := tx.Exec(ctx,
				`INSERT INTO modifier_options (group_id, name, price_delta, is_default, sort_order)
				 SELECT $1, $2, $3, $4, $5
				 WHERE NOT EXISTS (SELECT 1 FROM modifier_options WHERE group_id = $1 AND name = $2)`,
				id, option.name, option.delta, option.isDefault, optionIndex); err != nil {
				return nil, fmt.Errorf("opsi %s/%s: %w", group.name, option.name, err)
			}
		}
	}
	return ids, nil
}

func ensureIngredientCategory(ctx context.Context, tx pgx.Tx, businessID, name string) (string, error) {
	var id string
	err := tx.QueryRow(ctx,
		`INSERT INTO ingredient_categories (business_id, name) VALUES ($1, $2)
		 ON CONFLICT (business_id, lower(name)) DO UPDATE SET is_active = true
		 RETURNING id`, businessID, name).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("kategori bahan %s: %w", name, err)
	}
	return id, nil
}

func mustFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "seed-domain gagal: "+format+"\n", args...)
	os.Exit(1)
}
