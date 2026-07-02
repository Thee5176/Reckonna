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

  it('blurring clears the draft and falls back to the blurred (2dp grouped) display', () => {
    const { getByTestId } = render(<AmountInput testID="amt" value="4200" />);
    const input = getByTestId('amt');
    fireEvent(input, 'focus');
    fireEvent.changeText(input, '4200.5');
    fireEvent(input, 'blur');
    expect(input.props.value).toBe('4,200.00'); // draft cleared → reads off `value`, not the typed draft
  });

  it('a non-numeric value is treated as not-positive (invalid) and shown verbatim, not grouped', () => {
    const { getByTestId, getByText } = render(<AmountInput testID="amt" value="abc" />);
    expect(getByTestId('amt').props.value).toBe('abc'); // isNumericLike('abc') is false → no formatGrouped
    expect(getByText('Amount must be positive.')).toBeTruthy(); // isPositive short-circuits false on non-numeric input too
  });

  it('typing non-numeric garbage updates the draft but never fires onChangeValue', () => {
    const onChangeValue = jest.fn();
    const { getByTestId } = render(<AmountInput testID="amt" onChangeValue={onChangeValue} />);
    const input = getByTestId('amt');
    fireEvent(input, 'focus');
    fireEvent.changeText(input, 'abc');
    expect(input.props.value).toBe('abc');
    expect(onChangeValue).not.toHaveBeenCalled();
  });
});

function flatten(style: unknown): Record<string, unknown> {
  const arr = Array.isArray(style) ? style.flat(Infinity) : [style];
  return Object.assign({}, ...arr.filter(Boolean));
}
