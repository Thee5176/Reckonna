import React from 'react';
import { render, fireEvent } from '@testing-library/react-native';
import { Platform } from 'react-native';
import { AppNav } from './AppNav';

describe('AppNav (AT7 — adaptive chrome §08)', () => {
  it('AT7: platform=web renders the top navrow', () => {
    const { getByTestId } = render(<AppNav testID="nav" platform="web" />);
    expect(getByTestId('nav').props.accessibilityLabel).toBe('navrow');
  });

  it('AT7: platform=native renders the bottom tabbar', () => {
    const { getByTestId } = render(<AppNav testID="nav" platform="native" />);
    expect(getByTestId('nav').props.accessibilityLabel).toBe('tabbar');
  });

  it('AT7: both chromes expose the identical destinations', () => {
    const web = render(<AppNav testID="nav" platform="web" />);
    ['dashboard', 'ledger', 'record', 'reports'].forEach((k) =>
      expect(web.getByTestId(`nav-${k}`)).toBeTruthy(),
    );
    web.unmount();
    const native = render(<AppNav testID="nav" platform="native" />);
    ['dashboard', 'ledger', 'record', 'reports'].forEach((k) =>
      expect(native.getByTestId(`nav-${k}`)).toBeTruthy(),
    );
  });

  it('navigation fires onNavigate with the destination key and marks the active item', () => {
    const onNavigate = jest.fn();
    const { getByTestId } = render(
      <AppNav testID="nav" platform="web" activeKey="ledger" onNavigate={onNavigate} />,
    );
    expect(getByTestId('nav-ledger').props.accessibilityState.selected).toBe(true);
    fireEvent.press(getByTestId('nav-reports'));
    expect(onNavigate).toHaveBeenCalledWith('reports');
  });

  it('AT7: the native tabbar also fires onNavigate and marks the active tab', () => {
    const onNavigate = jest.fn();
    const { getByTestId } = render(
      <AppNav testID="nav" platform="native" activeKey="ledger" onNavigate={onNavigate} />,
    );
    expect(getByTestId('nav-ledger').props.accessibilityState.selected).toBe(true);
    fireEvent.press(getByTestId('nav-reports'));
    expect(onNavigate).toHaveBeenCalledWith('reports');
  });

  it('falls back to Platform.OS + default items/brand + default testID when unset', () => {
    const { getByTestId } = render(<AppNav />);
    expect(getByTestId('appnav')).toBeTruthy();
  });

  it('with no `platform` override, reads Platform.OS === "web" as the navrow chrome', () => {
    const original = Platform.OS;
    Platform.OS = 'web';
    try {
      const { getByTestId } = render(<AppNav testID="nav" />);
      expect(getByTestId('nav').props.accessibilityLabel).toBe('navrow');
    } finally {
      Platform.OS = original;
    }
  });
});
