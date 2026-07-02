import React from 'react';
import { render, fireEvent } from '@testing-library/react-native';
import { Button } from './Button';
import { color } from '../theme/tokens';

describe('Button (design §02 controls)', () => {
  it('renders the label and fires onPress when enabled', () => {
    const onPress = jest.fn();
    const { getByText } = render(<Button label="Post entry →" onPress={onPress} />);
    fireEvent.press(getByText('Post entry →'));
    expect(onPress).toHaveBeenCalledTimes(1);
  });

  it('primary variant is ink on ink', () => {
    const { getByTestId } = render(<Button testID="b" label="Post" variant="primary" />);
    const style = flatten(getByTestId('b').props.style);
    expect(style.backgroundColor).toBe(color.ink);
  });

  it('accent variant is the copper step-forward CTA', () => {
    const { getByTestId } = render(<Button testID="b" label="Review" variant="accent" />);
    const style = flatten(getByTestId('b').props.style);
    expect(style.backgroundColor).toBe(color.accent);
  });

  it('disabled is aria-disabled and does NOT fire onPress (unbalanced → cannot post)', () => {
    const onPress = jest.fn();
    const { getByTestId } = render(
      <Button testID="b" label="Post" onPress={onPress} disabled />,
    );
    const btn = getByTestId('b');
    expect(btn.props.accessibilityState.disabled).toBe(true);
    fireEvent.press(btn);
    expect(onPress).not.toHaveBeenCalled();
  });

  it.each(['primary', 'secondary', 'ghost', 'accent', 'danger'] as const)(
    'renders the %s variant',
    (variant) => {
      const { getByText } = render(<Button label={variant} variant={variant} />);
      expect(getByText(variant)).toBeTruthy();
    },
  );
});

function flatten(style: unknown): Record<string, unknown> {
  const arr = Array.isArray(style) ? style.flat(Infinity) : [style];
  return Object.assign({}, ...arr.filter(Boolean));
}
