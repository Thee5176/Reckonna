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

  it('Skeleton falls back to its default width when unset', () => {
    const { getByTestId } = render(<Skeleton testID="sk" />);
    const style = flatten(getByTestId('sk').props.style);
    expect(style.width).toBe('100%');
  });

  it('Spinner labels the loading state', () => {
    const { getByText, getByTestId } = render(<Spinner testID="sp" label="Fetching ledgers…" />);
    expect(getByTestId('sp').props.accessibilityLabel).toBe('loading');
    expect(getByText('Fetching ledgers…')).toBeTruthy();
  });

  it('Spinner renders bare (no label) without error', () => {
    const { getByTestId, queryByText } = render(<Spinner testID="sp" />);
    expect(getByTestId('sp').props.accessibilityLabel).toBe('loading');
    expect(queryByText(/./)).toBeNull();
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

  it('EmptyState omits the message and the action when neither is given', () => {
    const { getByTestId, queryByTestId, queryByText } = render(
      <EmptyState testID="empty" message="" />,
    );
    expect(getByTestId('empty')).toBeTruthy();
    expect(queryByTestId('empty-action')).toBeNull();
    expect(queryByText('Your first balanced entry will appear here.')).toBeNull();
  });

  it('EmptyState falls back to the default "empty" testID prefix on the action button', () => {
    const { getByTestId } = render(<EmptyState actionLabel="+ New entry" onAction={() => {}} />);
    expect(getByTestId('empty-action')).toBeTruthy();
  });
});

function flatten(style: unknown): Record<string, unknown> {
  const arr = Array.isArray(style) ? style.flat(Infinity) : [style];
  return Object.assign({}, ...arr.filter(Boolean));
}
