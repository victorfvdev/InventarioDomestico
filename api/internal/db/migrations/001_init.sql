-- Slice C: núcleo de estoque + carga inicial

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS households (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id),
    display_name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id),
    description TEXT NOT NULL,
    description_normalized TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT 'geral',
    unit TEXT NOT NULL DEFAULT 'un',
    quantity NUMERIC(14, 3) NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    min_quantity NUMERIC(14, 3),
    confirmation_status TEXT NOT NULL DEFAULT 'estimated'
        CHECK (confirmation_status IN ('estimated', 'confirmed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_products_household ON products(household_id);
CREATE INDEX IF NOT EXISTS idx_products_normalized ON products(household_id, description_normalized);

CREATE TABLE IF NOT EXISTS movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id),
    product_id UUID NOT NULL REFERENCES products(id),
    movement_type TEXT NOT NULL
        CHECK (movement_type IN (
            'manual_entry',
            'inventory_adjust',
            'initial_load',
            'purchase_entry',
            'consumption',
            'loss'
        )),
    quantity_delta NUMERIC(14, 3) NOT NULL,
    balance_after NUMERIC(14, 3) NOT NULL,
    origin TEXT NOT NULL,
    notes TEXT,
    is_consumption BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_movements_product ON movements(product_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_movements_household ON movements(household_id, created_at DESC);

CREATE TABLE IF NOT EXISTS ai_interpretations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id),
    source_type TEXT NOT NULL DEFAULT 'vision_photo',
    image_filename TEXT,
    provider TEXT NOT NULL,
    raw_response JSONB NOT NULL DEFAULT '{}'::jsonb,
    suggestions JSONB NOT NULL DEFAULT '[]'::jsonb,
    decision TEXT NOT NULL DEFAULT 'pending'
        CHECK (decision IN ('pending', 'confirmed', 'discarded')),
    overall_confidence NUMERIC(5, 4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ai_interp_household ON ai_interpretations(household_id, created_at DESC);
