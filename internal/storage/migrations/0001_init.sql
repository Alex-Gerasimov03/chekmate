-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE budgets (
    id         BIGSERIAL PRIMARY KEY,
    chat_id    BIGINT      NOT NULL UNIQUE,
    timezone   TEXT        NOT NULL DEFAULT 'Europe/Moscow',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE categories (
    id      BIGSERIAL PRIMARY KEY,
    code    TEXT    NOT NULL UNIQUE,
    title   TEXT    NOT NULL,
    is_food BOOLEAN NOT NULL
);

CREATE TABLE products (
    id              BIGSERIAL PRIMARY KEY,
    canonical_name  TEXT   NOT NULL,
    title           TEXT   NOT NULL DEFAULT '',
    fat             INT    NOT NULL DEFAULT 0,
    unit            TEXT   NOT NULL CHECK (unit IN ('kg', 'l', 'pcs')),
    pack_milli      BIGINT NOT NULL DEFAULT 0,
    category_id     BIGINT REFERENCES categories (id) ON DELETE SET NULL,
    -- Категорию мог проставить словарь или человек: словарь пересматривает
    -- только свои решения.
    category_source TEXT   NOT NULL DEFAULT 'auto' CHECK (category_source IN ('auto', 'user')),
    -- Канон зависит от словаря: по версии видно, что ключ пора пересчитать.
    dict_version    INT    NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Товар опознаётся четвёркой: название, жирность, единица и размер упаковки.
CREATE UNIQUE INDEX products_identity ON products (canonical_name, fat, unit, pack_milli);

-- Нечёткий поиск похожих названий: написание в чеках разнится от сети к сети.
CREATE INDEX products_name_trgm ON products USING gin (canonical_name gin_trgm_ops);

CREATE TABLE product_aliases (
    raw_name   TEXT PRIMARY KEY,
    product_id BIGINT  NOT NULL REFERENCES products (id) ON DELETE CASCADE,
    confirmed  BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX product_aliases_product_idx ON product_aliases (product_id);

CREATE TABLE receipts (
    id         BIGSERIAL PRIMARY KEY,
    budget_id  BIGINT      NOT NULL REFERENCES budgets (id) ON DELETE CASCADE,
    user_id    BIGINT      NOT NULL,
    fn         TEXT,
    fd         TEXT,
    fp         TEXT,
    total      BIGINT      NOT NULL,
    bought_at  TIMESTAMPTZ NOT NULL,
    merchant   TEXT        NOT NULL DEFAULT '',
    source     TEXT        NOT NULL CHECK (source IN ('qr', 'manual')),
    raw_qr     TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Частичный индекс не мешает ручным тратам, у которых фискальных полей нет.
CREATE UNIQUE INDEX receipts_fiscal_uniq ON receipts (fn, fd, fp) WHERE fn IS NOT NULL;
CREATE INDEX receipts_period_idx ON receipts (budget_id, bought_at DESC);
CREATE INDEX receipts_budget_idx ON receipts (budget_id);

CREATE TABLE receipt_items (
    id         BIGSERIAL PRIMARY KEY,
    receipt_id BIGINT NOT NULL REFERENCES receipts (id) ON DELETE CASCADE,
    raw_name   TEXT   NOT NULL,
    product_id BIGINT REFERENCES products (id) ON DELETE SET NULL,
    qty_milli  BIGINT NOT NULL,
    unit       TEXT   NOT NULL,
    sum        BIGINT NOT NULL,
    unit_price BIGINT NOT NULL
);

CREATE INDEX receipt_items_product_idx ON receipt_items (product_id);
CREATE INDEX receipt_items_receipt_idx ON receipt_items (receipt_id);

-- Словари вынесены из кода в данные: пополнять их можно без пересборки,
-- в том числе из бота.
CREATE TABLE abbreviations (
    short     TEXT PRIMARY KEY,
    expansion TEXT NOT NULL,
    source    TEXT NOT NULL DEFAULT 'seed' CHECK (source IN ('seed', 'user'))
);

CREATE TABLE stop_words (
    word   TEXT PRIMARY KEY,
    source TEXT NOT NULL DEFAULT 'seed' CHECK (source IN ('seed', 'user'))
);

CREATE TABLE category_rules (
    id          BIGSERIAL PRIMARY KEY,
    word        TEXT   NOT NULL,
    category_id BIGINT NOT NULL REFERENCES categories (id) ON DELETE CASCADE,
    budget_id   BIGINT REFERENCES budgets (id) ON DELETE CASCADE,
    priority    INT    NOT NULL DEFAULT 0,
    UNIQUE (word, budget_id)
);

CREATE INDEX category_rules_lookup ON category_rules (word);

-- +goose Down
DROP TABLE category_rules;
DROP TABLE stop_words;
DROP TABLE abbreviations;
DROP TABLE receipt_items;
DROP TABLE receipts;
DROP TABLE product_aliases;
DROP TABLE products;
DROP TABLE categories;
DROP TABLE budgets;
