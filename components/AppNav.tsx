// AppNav — adaptive navigation chrome (design §08). One destination list, two
// chromes: web (RN Web) renders a top `navrow`; native renders a bottom
// `tabbar` (RN Paper aesthetic). Identical logic, adaptive presentation (AT7).
// Branches on Platform.OS, overridable via `platform` for deterministic tests.
// Presentational: items + activeKey in, onNavigate out (routing is plan 05).
import React from 'react';
import { View, Text, Pressable, Platform, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { color, font } from '../theme/tokens';

export interface NavItem {
  key: string;
  label: string;
}

export interface AppNavProps {
  items?: NavItem[];
  activeKey?: string;
  onNavigate?: (key: string) => void;
  platform?: 'web' | 'native';
  brand?: string;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

// The canonical destinations — identical across platforms (design §08).
const DEFAULT_ITEMS: NavItem[] = [
  { key: 'dashboard', label: 'Dashboard' },
  { key: 'ledger', label: 'Ledger' },
  { key: 'record', label: 'Record' },
  { key: 'reports', label: 'Reports' },
];

export function AppNav({
  items = DEFAULT_ITEMS,
  activeKey,
  onNavigate,
  platform,
  brand = 'Reckonna',
  style,
  testID,
}: Readonly<AppNavProps>) {
  const isWeb = (platform ?? (Platform.OS === 'web' ? 'web' : 'native')) === 'web';
  const id = testID ?? 'appnav';

  if (isWeb) {
    return (
      <View testID={id} accessibilityLabel="navrow" style={[styles.topbar, style]}>
        <Text style={styles.brand}>{brand}</Text>
        <View style={styles.navrow}>
          {items.map((item) => {
            const active = item.key === activeKey;
            return (
              <Pressable
                key={item.key}
                testID={`${id}-${item.key}`}
                accessibilityRole="link"
                accessibilityState={{ selected: active }}
                onPress={() => onNavigate?.(item.key)}
              >
                <Text style={[styles.navLink, active && styles.navLinkActive]}>{item.label}</Text>
              </Pressable>
            );
          })}
        </View>
      </View>
    );
  }

  return (
    <View testID={id} accessibilityLabel="tabbar" style={[styles.tabbar, style]}>
      {items.map((item) => {
        const active = item.key === activeKey;
        return (
          <Pressable
            key={item.key}
            testID={`${id}-${item.key}`}
            accessibilityRole="button"
            accessibilityState={{ selected: active }}
            onPress={() => onNavigate?.(item.key)}
            style={[styles.tab, active && styles.tabActive]}
          >
            <Text style={[styles.tabLabel, active && styles.tabLabelActive]}>{item.label}</Text>
          </Pressable>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  topbar: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: 14,
    paddingHorizontal: 16,
    borderBottomWidth: 1,
    borderBottomColor: color.hairline,
    backgroundColor: color.surface,
  },
  brand: { fontFamily: font.serif, fontSize: 18, color: color.ink },
  navrow: { flexDirection: 'row', gap: 18 },
  navLink: { fontFamily: font.mono, fontSize: 12, color: color.ink3 },
  navLinkActive: { color: color.ink, borderBottomWidth: 2, borderBottomColor: color.ink, paddingBottom: 3 },
  tabbar: {
    flexDirection: 'row',
    borderTopWidth: 1,
    borderTopColor: color.hairline,
    backgroundColor: color.surface,
  },
  tab: { flex: 1, alignItems: 'center', paddingVertical: 10 },
  tabActive: { borderTopWidth: 2, borderTopColor: color.ink, marginTop: -1 },
  tabLabel: { fontFamily: font.mono, fontSize: 9.5, color: color.ink3, letterSpacing: 0.6, textTransform: 'uppercase' },
  tabLabelActive: { color: color.ink },
});
