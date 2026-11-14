-- F08 Resep per varian produk, versi terpisah; hanya satu ACTIVE per varian.

CREATE TABLE recipes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    variant_id UUID NOT NULL REFERENCES product_variants(id) ON DELETE RESTRICT,
    version_no INT NOT NULL DEFAULT 1 CHECK (version_no > 0),
    status TEXT NOT NULL DEFAULT 'ACTIVE' CHECK (status IN ('ACTIVE', 'INACTIVE')),
    yield_note TEXT,
    effective_from DATE,
    effective_to DATE,
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_recipes_variant_version UNIQUE (variant_id, version_no)
);
CREATE UNIQUE INDEX uq_recipes_variant_active ON recipes (variant_id) WHERE status = 'ACTIVE';

-- Hanya tabel anak murni resep (draft) yang CASCADE.
CREATE TABLE recipe_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipe_id UUID NOT NULL REFERENCES recipes(id) ON DELETE CASCADE,
    ingredient_id UUID NOT NULL REFERENCES ingredients(id) ON DELETE RESTRICT,
    quantity NUMERIC(18,4) NOT NULL CHECK (quantity > 0),
    waste_factor_percent NUMERIC(7,4) NOT NULL DEFAULT 0 CHECK (waste_factor_percent >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_recipe_items_recipe_ingredient UNIQUE (recipe_id, ingredient_id)
);
CREATE INDEX idx_recipe_items_ingredient ON recipe_items (ingredient_id);
