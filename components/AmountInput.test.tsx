import React from 'react';
import { render, fireEvent } from '@testing-library/react-native';
import { AmountInput } from './AmountInput';
import { color } from '../theme/tokens';

describe('AmountInput (§03 — tabular, 4dp under the hood)', () => {
  it('AT3: a negative amount is invalid with the backend-mirrored message', () => {
    const { getByTestId, getByText } = render(<AmountInput testID="amt" value="-50.00" />);
    const style = flatten(getByTestId('amt').props.style);
    expect(style.borderColor).toBe(color.debit);
    expect(getByText('Amount must be positive.')).toBeTruthy();
  });

  it('IT7: onChangeValue emits the canonical 4dp string, not a JS number', () => {
    const onChangeValue = jest.fn();
    const { getByTestId } = render(<AmountInput testID="amt" onChangeValue={onChangeValue} />);
    const input = getByTestId('amt');
    fireEvent(input, 'focus');
    fireEvent.changeText(input, '384');
    expect(onChangeValue).toHaveBeenLastCalledWith('384.0000');
    fireEvent.changeText(input, '4200.5');
    expect(onChangeValue).toHaveBeenLastCalledWith('4200.5000');
  });

  it('IT7: blurred display rounds to 2dp grouped; focused reveals 4dp', () => {
    const { getByTestId, rerender } = render(<AmountInput testID="amt" value="4200" />);
    expect(getByTestId('amt').props.value).toBe('4,200.00'); // blurred: 2dp grouped
    fireEvent(getByTestId('amt'), 'focus');
    rerender(<AmountInput testID="amt" value="4200" />);
    expect(getByTestId('amt').props.value).toBe('4200.0000'); // focused: 4dp
  });

  it('a positive amount is not invalid', () => {
    const { queryByText } = render(<AmountInput value="384.00" />);
    expect(queryByText('Amount must be positive.')).toBeNull();
  });
});

function flatten(style: unknown): Record<string, unknown> {
  const arr = Array.isArray(style) ? style.flat(Infinity) : [style];
  return Object.assign({}, ...arr.filter(Boolean));
}
