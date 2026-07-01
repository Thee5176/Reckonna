import React from 'react';
import { render } from '@testing-library/react-native';
import { Badge } from './Badge';
import { color } from '../theme/tokens';

describe('Badge (entry status §02)', () => {
  it.each([
    ['draft', color.ink3],
    ['review', color.accent],
    ['posted', color.credit],
    ['flagged', color.debit],
  ] as const)('%s badge uses its lifecycle color', (status, expected) => {
    const { getByText } = render(<Badge label={status} status={status} />);
    const style = flatten(getByText(status).props.style);
    expect(style.color).toBe(expected);
  });

  it('renders arbitrary labels (used as the 借方/貸方 line marker too)', () => {
    const { getByText } = render(<Badge label="借方" status="flagged" />);
    expect(getByText('借方')).toBeTruthy();
  });
});

function flatten(style: unknown): Record<string, unknown> {
  const arr = Array.isArray(style) ? style.flat(Infinity) : [style];
  return Object.assign({}, ...arr.filter(Boolean));
}
