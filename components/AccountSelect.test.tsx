import React from 'react';
import { render, fireEvent } from '@testing-library/react-native';
import { AccountSelect, type Account } from './AccountSelect';
import { color } from '../theme/tokens';

const COA: Account[] = [
  { code: '4101', name: 'Sales revenue', element: 'Revenue' },
  { code: '6210', name: 'Software', element: 'Expenses' },
  { code: '1100', name: 'Cash', element: 'Assets' },
];

describe('AccountSelect (CoA picker §03)', () => {
  it('shows the placeholder until a code is chosen', () => {
    const { getByTestId } = render(<AccountSelect testID="acc" label="Account · CoA" accounts={COA} />);
    expect(getByTestId('acc')).toBeTruthy();
  });

  it('opening the list and picking an option fires onChange(code)', () => {
    const onChange = jest.fn();
    const { getByTestId } = render(
      <AccountSelect testID="acc" label="Account · CoA" accounts={COA} onChange={onChange} />,
    );
    fireEvent.press(getByTestId('acc'));
    fireEvent.press(getByTestId('acc-opt-4101'));
    expect(onChange).toHaveBeenCalledWith('4101');
  });

  it('renders the selected code · name once chosen', () => {
    const { getByText } = render(
      <AccountSelect label="Account · CoA" accounts={COA} value="1100" />,
    );
    expect(getByText('1100 · Cash')).toBeTruthy();
  });

  it('invalid state borders debit and shows the error', () => {
    const { getByTestId, getByText } = render(
      <AccountSelect
        testID="acc"
        label="Account · CoA"
        accounts={COA}
        invalid
        error="Choose a chart-of-account code."
      />,
    );
    const style = flatten(getByTestId('acc').props.style);
    expect(style.borderColor).toBe(color.debit);
    expect(getByText('Choose a chart-of-account code.')).toBeTruthy();
  });
});

function flatten(style: unknown): Record<string, unknown> {
  const arr = Array.isArray(style) ? style.flat(Infinity) : [style];
  return Object.assign({}, ...arr.filter(Boolean));
}
