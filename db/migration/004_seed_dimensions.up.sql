-- 004_seed_dimensions.up.sql — base book + v1 dimension types/values (plan 03 S5c).
-- v1 = the single `base` book (standard §8 R8.1). Dimension types per §7:
-- entity, currency, counterparty. Currency members are ISO 4217; JPY is the
-- functional default. Members are data (§7 R7.2) — extend without a CoA change.

INSERT INTO book (code, name) VALUES ('base', 'Base book');

INSERT INTO dimension_type (code, name) VALUES
    ('entity',       'Entity'),
    ('currency',     'Currency'),
    ('counterparty', 'Counterparty');

-- currency members (JPY functional default)
INSERT INTO dimension_value (dimension_type_id, code, name)
SELECT dt.id, v.code, v.name
  FROM dimension_type dt
  CROSS JOIN (VALUES
        ('JPY', 'Japanese yen'),
        ('USD', 'US dollar'),
        ('EUR', 'Euro'),
        ('GBP', 'Pound sterling')
  ) AS v(code, name)
 WHERE dt.code = 'currency';

-- default single entity (§7 R7.5 — daily entry uses the default so the user never sees this)
INSERT INTO dimension_value (dimension_type_id, code, name)
SELECT id, 'default', 'Default entity' FROM dimension_type WHERE code = 'entity';

-- one generic counterparty so required-dimension postings (e.g. 21500 escrow) have a value to reference
INSERT INTO dimension_value (dimension_type_id, code, name)
SELECT id, 'external', 'External counterparty' FROM dimension_type WHERE code = 'counterparty';
