import React from 'react';
import { render } from '@testing-library/react-native';
import {
  Button, Badge, Field, AccountSelect, DebitCreditSegment, AmountInput,
  Skeleton, Spinner, EmptyState, Alert, BalanceBar, LedgerTable,
  JournalEntryForm, StatementTable, AppNav,
  tokens, color, font, radius, shadow, paperTheme, withAlpha, money,
} from './index';

describe('component library barrel (S17)', () => {
  it('exports every public component + the theme/money helpers', () => {
    const surface = {
      Button, Badge, Field, AccountSelect, DebitCreditSegment, AmountInput,
      Skeleton, Spinner, EmptyState, Alert, BalanceBar, LedgerTable,
      JournalEntryForm, StatementTable, AppNav,
      tokens, color, font, radius, shadow, paperTheme, withAlpha, money,
    };
    Object.entries(surface).forEach(([, value]) => expect(value).toBeDefined());
  });

  it('money re-export is the decimal-exact helper (no float drift)', () => {
    expect(money.sum(['1000.0000', '-500.0000'])).toBe('500.0000');
  });
});

describe('design-parity snapshots (S17)', () => {
  it('Button variants', () => {
    const primary = render(<Button label="Post entry →" variant="primary" />).toJSON();
    const accent = render(<Button label="Review balance →" variant="accent" />).toJSON();
    const disabled = render(<Button label="Post entry →" disabled />).toJSON();
    expect(primary).toMatchSnapshot('button-primary');
    expect(accent).toMatchSnapshot('button-accent');
    expect(disabled).toMatchSnapshot('button-disabled');
  });

  it('Badge lifecycle', () => {
    expect(render(<Badge label="posted" status="posted" />).toJSON()).toMatchSnapshot('badge-posted');
  });

  it('BalanceBar ok vs bad', () => {
    const ok = render(<BalanceBar debits={['1000']} credits={['1000']} />).toJSON();
    const bad = render(<BalanceBar debits={['1000']} credits={['500']} />).toJSON();
    expect(ok).toMatchSnapshot('balancebar-ok');
    expect(bad).toMatchSnapshot('balancebar-bad');
  });
});
