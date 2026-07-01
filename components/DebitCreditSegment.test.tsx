import React from 'react';
import { render, fireEvent } from '@testing-library/react-native';
import { DebitCreditSegment } from './DebitCreditSegment';
import { color } from '../theme/tokens';

describe('DebitCreditSegment (AT8 — 借方/貸方 toggle §03)', () => {
  it('defaults to 借方 (debit) selected', () => {
    const { getByTestId } = render(<DebitCreditSegment testID="seg" />);
    expect(getByTestId('seg-debit').props.accessibilityState.selected).toBe(true);
    expect(getByTestId('seg-credit').props.accessibilityState.selected).toBe(false);
  });

  it('tapping 貸方 fires onChange("credit") and recolors the active segment to --credit', () => {
    const onChange = jest.fn();
    const { getByTestId, getByText } = render(
      <DebitCreditSegment testID="seg" value="credit" onChange={onChange} />,
    );
    fireEvent.press(getByTestId('seg-credit'));
    expect(onChange).toHaveBeenCalledWith('credit');
    const style = flatten(getByText('貸方 Credit').props.style);
    expect(style.color).toBe(color.credit);
  });

  it('debit segment when active is tinted --debit', () => {
    const { getByText } = render(<DebitCreditSegment value="debit" />);
    const style = flatten(getByText('借方 Debit').props.style);
    expect(style.color).toBe(color.debit);
  });
});

function flatten(style: unknown): Record<string, unknown> {
  const arr = Array.isArray(style) ? style.flat(Infinity) : [style];
  return Object.assign({}, ...arr.filter(Boolean));
}
