import React from 'react';
import { render } from '@testing-library/react-native';
import { BalanceBar } from './BalanceBar';
import { color } from '../theme/tokens';

describe('BalanceBar (IT3 — 借方=貸方 derived, not passed §05)', () => {
  it('IT3: exactly-equal decimal strings compute ok', () => {
    const { getByTestId, getByText } = render(
      <BalanceBar testID="bar" debits={['4200.00']} credits={['4200.00']} />,
    );
    expect(getByTestId('bar').props.accessibilityLabel).toBe('balanced');
    expect(getByText('✓ Balanced')).toBeTruthy();
  });

  it('IT3: a 0.0001 mismatch computes bad (unbalanced case)', () => {
    const { getByTestId, getByText } = render(
      <BalanceBar testID="bar" debits={['1000.0001']} credits={['1000.0000']} />,
    );
    expect(getByTestId('bar').props.accessibilityLabel).toBe('unbalanced');
    expect(getByText('✕ 借方≠貸方')).toBeTruthy();
  });

  it('AT1: balanced → Difference 0.00 and the CTA is ENABLED', () => {
    const { getByTestId, getByText } = render(
      <BalanceBar testID="bar" debits={['1000']} credits={['1000']} ctaLabel="Review balance →" />,
    );
    expect(getByText('0.00')).toBeTruthy();
    expect(getByTestId('bar-cta').props.accessibilityState.disabled).toBe(false);
  });

  it('AT2: unbalanced → Difference 500.00 (debit-colored) and the CTA is DISABLED', () => {
    const { getByTestId, getByText } = render(
      <BalanceBar testID="bar" debits={['1000']} credits={['500']} ctaLabel="Review balance →" />,
    );
    const diff = getByText('500.00');
    const style = flatten(diff.props.style);
    expect(style.color).toBe(color.debit);
    expect(getByTestId('bar-cta').props.accessibilityState.disabled).toBe(true);
  });

  it('cannot be told it is balanced — there is no boolean prop, only debits/credits', () => {
    // Passing wildly unequal sums must still read unbalanced.
    const { getByTestId } = render(<BalanceBar testID="bar" debits={['9']} credits={['1']} />);
    expect(getByTestId('bar').props.accessibilityLabel).toBe('unbalanced');
  });
});

function flatten(style: unknown): Record<string, unknown> {
  const arr = Array.isArray(style) ? style.flat(Infinity) : [style];
  return Object.assign({}, ...arr.filter(Boolean));
}
