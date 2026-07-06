import React from 'react';
import { ScrollView, StyleSheet } from 'react-native';
import { JournalEntryForm } from '../components/JournalEntryForm';
import type { Account } from '../components/AccountSelect';
import { color } from '../theme/tokens';

const COA: Account[] = [
  { code: '1100', name: 'Cash', element: 'Assets' },
  { code: '4101', name: 'Sales revenue', element: 'Revenue' },
];

export default function Home() {
  return (
    <ScrollView contentContainerStyle={styles.page}>
      <JournalEntryForm
        accounts={COA}
        initialDate="2026-05-24"
        initialDescription="Stripe payout · 14 invoices"
      />
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  page: { padding: 24, backgroundColor: color.bg, minHeight: '100%' },
});
