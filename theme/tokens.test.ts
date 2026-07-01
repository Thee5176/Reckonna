// IT1 — token-drift gate. These literals are copied from the design system
// :root block in design/02-frontend-ledger.design-system.html (Forest Reserve,
// lines 19–29). If a designer edits the HTML, this test must be updated in
// lockstep — a drift in any token fails the build (plan 04 decision #2, IT1).
import { tokens } from './tokens';

describe('IT1 — Forest Reserve token parity vs design HTML', () => {
  it('surfaces + ink match the :root custom properties', () => {
    expect(tokens.color.bg).toBe('#efece1');
    expect(tokens.color.bgElev).toBe('#e2dccb');
    expect(tokens.color.surface).toBe('#faf7ec');
    expect(tokens.color.ink).toBe('#11201a');
    expect(tokens.color.ink2).toBe('#324138');
    expect(tokens.color.ink3).toBe('#6d6b58');
  });

  it('rules + hairlines match', () => {
    expect(tokens.color.rule).toBe('#1b2a23');
    expect(tokens.color.ruleSoft).toBe('rgba(17,32,26,0.12)');
    expect(tokens.color.hairline).toBe('rgba(17,32,26,0.07)');
  });

  it('debit/credit/accent/highlight are the ledger semantics', () => {
    expect(tokens.color.debit).toBe('#7a1d1d');
    expect(tokens.color.credit).toBe('#2a5a2a');
    expect(tokens.color.accent).toBe('#8a5a1c');
    expect(tokens.color.highlight).toBe('#e9d391');
  });

  it('state aliases map onto existing semantics (--ok≡credit, --bad≡debit, --warn≡accent)', () => {
    expect(tokens.color.ok).toBe(tokens.color.credit);
    expect(tokens.color.bad).toBe(tokens.color.debit);
    expect(tokens.color.warn).toBe(tokens.color.accent);
    expect(tokens.color.ok).toBe('#2a5a2a');
    expect(tokens.color.bad).toBe('#7a1d1d');
  });

  it('font families are the bundled Source Serif 4 + JetBrains Mono', () => {
    expect(tokens.font.serif).toBe('Source Serif 4');
    expect(tokens.font.mono).toBe('JetBrains Mono');
  });

  it('radii are sm/md/lg = 4/6/10 (no pill shapes — this is a ledger)', () => {
    expect(tokens.radius.sm).toBe(4);
    expect(tokens.radius.md).toBe(6);
    expect(tokens.radius.lg).toBe(10);
  });
});
