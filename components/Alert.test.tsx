import React from 'react';
import { render, fireEvent } from '@testing-library/react-native';
import { Alert } from './Alert';

describe('Alert (AT5 / IT5 — RFC 7807 code-keyed §04)', () => {
  it('AT5/IT5: a 422 unbalanced_entry renders the inline balance Alert (by code)', () => {
    const { getByTestId } = render(<Alert code="unbalanced_entry" />);
    // Assert on the code, never localized text (locale-fragile, IT5).
    expect(getByTestId('alert-unbalanced_entry').props.accessibilityLabel).toBe('unbalanced_entry');
  });

  it('AT5: a 5xx server_error renders a Retry Alert', () => {
    const onRetry = jest.fn();
    const { getByTestId } = render(<Alert code="server_error" onRetry={onRetry} />);
    expect(getByTestId('alert-server_error')).toBeTruthy();
    fireEvent.press(getByTestId('alert-server_error-retry'));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('IT5: validation_failed + unauthorized map to their own code, no Retry', () => {
    const { getByTestId, queryByTestId, rerender } = render(<Alert code="validation_failed" />);
    expect(getByTestId('alert-validation_failed').props.accessibilityLabel).toBe('validation_failed');
    expect(queryByTestId('alert-validation_failed-retry')).toBeNull();

    rerender(<Alert code="unauthorized" />);
    expect(getByTestId('alert-unauthorized').props.accessibilityLabel).toBe('unauthorized');
    expect(queryByTestId('alert-unauthorized-retry')).toBeNull();
  });

  it('non-retryable codes do not render Retry even if onRetry is passed', () => {
    const { queryByTestId } = render(<Alert code="unbalanced_entry" onRetry={jest.fn()} />);
    expect(queryByTestId('alert-unbalanced_entry-retry')).toBeNull();
  });
});
