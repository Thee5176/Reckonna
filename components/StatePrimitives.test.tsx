import React from 'react';
import { render, fireEvent } from '@testing-library/react-native';
import { Skeleton } from './Skeleton';
import { Spinner } from './Spinner';
import { EmptyState } from './EmptyState';

describe('State primitives (§04 Loading / Empty)', () => {
  it('Skeleton is a loading placeholder', () => {
    const { getByTestId } = render(<Skeleton testID="sk" width="60%" />);
    expect(getByTestId('sk').props.accessibilityLabel).toBe('loading');
  });

  it('Spinner labels the loading state', () => {
    const { getByText, getByTestId } = render(<Spinner testID="sp" label="Fetching ledgers…" />);
    expect(getByTestId('sp').props.accessibilityLabel).toBe('loading');
    expect(getByText('Fetching ledgers…')).toBeTruthy();
  });

  it('EmptyState shows the CoA-grid copy + optional accent action', () => {
    const onAction = jest.fn();
    const { getByText, getByTestId } = render(
      <EmptyState testID="empty" actionLabel="+ New entry" onAction={onAction} />,
    );
    expect(getByTestId('empty').props.accessibilityLabel).toBe('empty');
    expect(getByText('No entries yet.')).toBeTruthy();
    fireEvent.press(getByTestId('empty-action'));
    expect(onAction).toHaveBeenCalledTimes(1);
  });
});
