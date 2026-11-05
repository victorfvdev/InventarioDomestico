-- EST-4: documentos fiscais NFC-e (MT) + itens para purchase_entry

CREATE TABLE IF NOT EXISTS fiscal_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    household_id UUID NOT NULL REFERENCES households(id),
    access_key TEXT NOT NULL,
    uf CHAR(2) NOT NULL DEFAULT 'MT',
    document_type TEXT NOT NULL DEFAULT 'nfce'
        CHECK (document_type IN ('nfce')),
    issuer_name TEXT NOT NULL DEFAULT '',
    issuer_cnpj TEXT NOT NULL DEFAULT '',
    issued_at TIMESTAMPTZ,
    total_amount NUMERIC(14, 2) NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'fetched', 'parsed', 'confirmed', 'rejected')),
    source TEXT NOT NULL DEFAULT 'manual'
        CHECK (source IN ('fixture', 'sefaz', 'qr', 'manual', 'photo_stub')),
    raw_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    fetch_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at TIMESTAMPTZ,
    CONSTRAINT fiscal_documents_access_key_len CHECK (
        access_key = '' OR (char_length(access_key) = 44 AND access_key ~ '^[0-9]{44}$')
    )
);

-- Idempotência: mesma chave na residência não gera segunda confirmação.
-- Chave vazia (entrada 100% manual sem chave) permite várias linhas.
CREATE UNIQUE INDEX IF NOT EXISTS uq_fiscal_documents_household_access_key
    ON fiscal_documents(household_id, access_key)
    WHERE access_key <> '';

CREATE INDEX IF NOT EXISTS idx_fiscal_documents_household
    ON fiscal_documents(household_id, created_at DESC);

CREATE TABLE IF NOT EXISTS fiscal_document_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fiscal_document_id UUID NOT NULL REFERENCES fiscal_documents(id) ON DELETE CASCADE,
    line_number INT NOT NULL,
    description TEXT NOT NULL,
    description_normalized TEXT NOT NULL,
    quantity NUMERIC(14, 3) NOT NULL CHECK (quantity >= 0),
    unit TEXT NOT NULL DEFAULT 'UN',
    unit_price NUMERIC(14, 4) NOT NULL DEFAULT 0,
    total_price NUMERIC(14, 2) NOT NULL DEFAULT 0,
    ean TEXT,
    category TEXT NOT NULL DEFAULT 'geral',
    match_status TEXT NOT NULL DEFAULT 'new'
        CHECK (match_status IN ('exact', 'suggested', 'ambiguous', 'new', 'manual')),
    match_confidence NUMERIC(5, 4),
    matched_product_id UUID REFERENCES products(id),
    create_new BOOLEAN NOT NULL DEFAULT true,
    included BOOLEAN NOT NULL DEFAULT true,
    confirmed_product_id UUID REFERENCES products(id),
    movement_id UUID REFERENCES movements(id),
    UNIQUE (fiscal_document_id, line_number)
);

CREATE INDEX IF NOT EXISTS idx_fiscal_items_document
    ON fiscal_document_items(fiscal_document_id);
