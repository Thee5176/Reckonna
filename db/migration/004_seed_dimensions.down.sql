-- 004_seed_dimensions.down.sql — reverse of 004 (values before types, via FK).
DELETE FROM dimension_value
 WHERE dimension_type_id IN (SELECT id FROM dimension_type WHERE code IN ('entity', 'currency', 'counterparty'));
DELETE FROM dimension_type WHERE code IN ('entity', 'currency', 'counterparty');
DELETE FROM book WHERE code = 'base';
