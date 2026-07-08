// color.ts — alpha-variant + debit/credit semantic pair utilities (design
// §01: no new hue, only alpha variants / pairings of existing tokens.color
// values).
import { withAlpha, forType } from './color';
import { color } from './tokens';

describe('withAlpha (token → rgba, no new hue)', () => {
  it('derives an rgba() of the given token hex at the given alpha', () => {
    expect(withAlpha('#7a1d1d', 0.3)).toBe('rgba(122,29,29,0.3)');
  });

  it('handles alpha 0 and 1 at the boundary', () => {
    expect(withAlpha('#000000', 0)).toBe('rgba(0,0,0,0)');
    expect(withAlpha('#ffffff', 1)).toBe('rgba(255,255,255,1)');
  });
});

describe('forType (debit|credit → the one place color is chosen)', () => {
  it('debit maps to color.debit', () => {
    expect(forType('debit')).toBe(color.debit);
  });

  it('credit maps to color.credit', () => {
    expect(forType('credit')).toBe(color.credit);
  });
});
