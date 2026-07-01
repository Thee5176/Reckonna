// IT2 — decimal-exact money. Amounts are strings end-to-end (mirror backend
// NUMERIC(20,4)); NO JS number arithmetic on money (IEEE-754 float is exactly
// the precision bug plan 03 killed on the backend). Boundary cases below prove
// no float drift. Rounding policy: half away from zero (matches Postgres
// NUMERIC ROUND), documented in money.ts.
import { toValue, format, formatGrouped, sum, difference, isBalanced } from './money';

describe('IT2 — money: value precision (4dp store)', () => {
  it('normalizes to 4dp, half-up on the 5th decimal', () => {
    // 0.12345 → 4dp value keeps 0.1235 (5th digit 5 rounds up)
    expect(toValue('0.12345')).toBe('0.1235');
    expect(toValue('1000')).toBe('1000.0000');
    expect(toValue('4200.00')).toBe('4200.0000');
  });

  it('never uses float arithmetic — sums exact decimal strings', () => {
    expect(sum(['1000.0000', '-500.0000'])).toBe('500.0000');
    expect(sum(['0.1', '0.2'])).toBe('0.3000'); // classic float trap 0.1+0.2
    expect(sum([])).toBe('0.0000');
    expect(sum(['4200.00', '-4200.00'])).toBe('0.0000');
  });
});

describe('IT2 — money: display (2dp)', () => {
  it('rounds display to 2dp half-up under a documented policy', () => {
    expect(format('1000.33335')).toBe('1000.33'); // 3rd decimal 3 → down
    expect(format('1000.335')).toBe('1000.34'); // 3rd decimal 5 → up
    expect(format('4200')).toBe('4200.00');
    expect(format('-50')).toBe('-50.00');
  });

  it('groups thousands for UI display (design: 4,200.00)', () => {
    expect(formatGrouped('4200')).toBe('4,200.00');
    expect(formatGrouped('1763.4')).toBe('1,763.40');
    expect(formatGrouped('96850.18')).toBe('96,850.18');
  });
});

describe('IT2 — money: balance (借方=貸方 gate)', () => {
  it('difference is signed debit − credit at 4dp', () => {
    expect(difference(['1000'], ['500'])).toBe('500.0000');
    expect(difference(['1000'], ['1000'])).toBe('0.0000');
  });

  it('isBalanced is true only when sums are exactly equal', () => {
    expect(isBalanced(['4200.00'], ['4200.00'])).toBe(true);
    expect(isBalanced(['1000', '200'], ['1200'])).toBe(true);
  });

  it('UNBALANCED case: 借方≠貸方 fails, incl. a 0.0001 mismatch', () => {
    expect(isBalanced(['1000'], ['500'])).toBe(false);
    expect(isBalanced(['1000.0001'], ['1000.0000'])).toBe(false);
  });
});
