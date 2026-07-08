import React from 'react';
import { render, fireEvent } from '@testing-library/react-native';
import { Field } from './Field';
import { color } from '../theme/tokens';

describe('Field (§03 inputs)', () => {
  it('renders label + value and emits onChangeText', () => {
    const onChangeText = jest.fn();
    const { getByTestId, getByText } = render(
      <Field testID="f" label="Description" value="Stripe payout" onChangeText={onChangeText} />,
    );
    expect(getByText('Description')).toBeTruthy();
    fireEvent.changeText(getByTestId('f'), 'Linear seat');
    expect(onChangeText).toHaveBeenCalledWith('Linear seat');
  });

  it('invalid state borders debit and shows the error line', () => {
    const { getByTestId, getByText } = render(
      <Field testID="f" label="Amount · JPY" value="-50.00" invalid error="Amount must be positive." />,
    );
    const style = flatten(getByTestId('f').props.style);
    expect(style.borderColor).toBe(color.debit);
    expect(getByText('Amount must be positive.')).toBeTruthy();
  });

  it('valid field hides the error', () => {
    const { queryByText } = render(
      <Field label="Amount" value="384.00" error="Amount must be positive." />,
    );
    expect(queryByText('Amount must be positive.')).toBeNull();
  });
});

function flatten(style: unknown): Record<string, unknown> {
  const arr = Array.isArray(style) ? style.flat(Infinity) : [style];
  return Object.assign({}, ...arr.filter(Boolean));
}
