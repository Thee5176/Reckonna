// JournalEntryForm — the two-step journal capture (design §05). Step 1 reads
// the entry "as a sentence" over a debit/credit line table with a live
// BalanceBar; step 2 reviews before posting. The Post/Review CTA is dead until
// 借方=貸方 (the BalanceBar computes this — AT1/AT2). PRESENTATIONAL: onSubmit
// emits the plan-03 POST /command/journal-entries payload; the form NEVER calls
// a network API itself (no axios/fetch here — hooks + live wiring are plan 05,
// IT4). The signed-in owner is injected server-side, not in this payload.
import React, { useRef, useState } from 'react';
import { View, Text, Pressable, StyleSheet } from 'react-native';
import type { StyleProp, ViewStyle } from 'react-native';
import { color, font, radius } from '../theme/tokens';
import { withAlpha } from '../theme/color';
import { toValue, formatGrouped } from '../lib/money';
import { AccountSelect, type Account } from './AccountSelect';
import { AmountInput } from './AmountInput';
import { DebitCreditSegment, type EntryType } from './DebitCreditSegment';
import { BalanceBar } from './BalanceBar';
import { Button } from './Button';
import { Badge } from './Badge';
import { Field } from './Field';
import { Alert, type ErrorCode } from './Alert';

export interface JournalLineInput {
  account: string; // CoA code
  amount: string; // decimal string
  side: EntryType; // debit | credit
}

export interface JournalEntryPayload {
  date: string;
  description: string;
  book: string;
  lines: JournalLineInput[];
}

export interface JournalEntryFormProps {
  accounts: Account[];
  initialDate?: string;
  initialDescription?: string;
  initialLines?: JournalLineInput[];
  book?: string;
  onSubmit?: (payload: JournalEntryPayload) => void;
  onSaveDraft?: (payload: JournalEntryPayload) => void;
  errorCode?: ErrorCode;
  onRetry?: () => void;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

const EMPTY_LINE: JournalLineInput = { account: '', amount: '', side: 'debit' };

// A safe, numeric-only amount for the balance math (blank / partial → '0').
function safeAmount(a: string): string {
  return /^[+-]?\d*\.?\d+$/.test(a) ? a : '0';
}

// A line tagged with a stable render key — account codes can repeat across
// lines (two blank lines, two lines on the same account), so the array index
// is the only "identity" a line has otherwise. `_key` never leaves this
// component: buildPayload() below only ever reads account/amount/side.
type KeyedLine = JournalLineInput & { readonly _key: number };

export function JournalEntryForm({
  accounts,
  initialDate = '',
  initialDescription = '',
  initialLines,
  book = 'base',
  onSubmit,
  onSaveDraft,
  errorCode,
  onRetry,
  style,
  testID,
}: Readonly<JournalEntryFormProps>) {
  const [date, setDate] = useState(initialDate);
  const [description, setDescription] = useState(initialDescription);
  const nextKey = useRef(0);
  function withKey(line: JournalLineInput): KeyedLine {
    nextKey.current += 1;
    return { ...line, _key: nextKey.current };
  }
  const [lines, setLines] = useState<KeyedLine[]>(() =>
    (initialLines?.length ? initialLines : [{ ...EMPTY_LINE }]).map(withKey),
  );
  const [step, setStep] = useState<1 | 2>(1);

  const debits = lines.filter((l) => l.side === 'debit').map((l) => safeAmount(l.amount));
  const credits = lines.filter((l) => l.side === 'credit').map((l) => safeAmount(l.amount));

  const id = testID ?? 'jef';

  function patchLine(index: number, patch: Partial<JournalLineInput>) {
    setLines((prev) => prev.map((l, i) => (i === index ? { ...l, ...patch } : l)));
  }
  function addLine() {
    setLines((prev) => [...prev, withKey({ ...EMPTY_LINE })]);
  }

  function buildPayload(): JournalEntryPayload {
    return {
      date,
      description,
      book,
      lines: lines.map((l) => ({
        account: l.account,
        amount: toValue(safeAmount(l.amount)),
        side: l.side,
      })),
    };
  }

  return (
    <View testID={id} style={[styles.form, style]}>
      {/* step indicator (design §05 navrow) */}
      <View style={styles.navrow}>
        <StepTab label="1 · Entry" active={step === 1} onPress={() => setStep(1)} />
        <StepTab label="2 · Review & post" active={step === 2} onPress={undefined} />
      </View>

      {errorCode ? (
        <Alert code={errorCode} onRetry={onRetry} style={styles.alert} />
      ) : null}

      {step === 1 ? (
        <>
          {/* sentence-shaped capture */}
          <View style={styles.card}>
            <Text style={styles.eyebrow}>Sentence-shaped capture</Text>
            <Text style={styles.sentence}>
              On <Text style={styles.hl}>{date || '—'}</Text> we recorded{' '}
              <Text style={styles.hl}>{description || '—'}</Text> across{' '}
              <Text style={styles.hlNum}>{lines.length}</Text> lines.
            </Text>
            <View style={styles.captureFields}>
              <Field
                testID={`${id}-date`}
                label="Date"
                value={date}
                onChangeText={setDate}
                placeholder="2026-05-24"
                style={styles.grow}
              />
              <Field
                testID={`${id}-description`}
                label="Description"
                value={description}
                onChangeText={setDescription}
                placeholder="Stripe payout · 14 invoices"
                style={styles.growWide}
              />
            </View>
          </View>

          {/* line items */}
          <View style={styles.lines}>
            {lines.map((line, i) => (
              <View key={line._key} testID={`${id}-line-${i}`} style={styles.lineRow}>
                <DebitCreditSegment
                  testID={`${id}-line-${i}-side`}
                  value={line.side}
                  onChange={(side) => patchLine(i, { side })}
                />
                <AccountSelect
                  testID={`${id}-line-${i}-account`}
                  label="Account · CoA"
                  accounts={accounts}
                  value={line.account}
                  onChange={(account) => patchLine(i, { account })}
                  style={styles.grow}
                />
                <AmountInput
                  testID={`${id}-line-${i}-amount`}
                  value={line.amount}
                  onChangeValue={(amount) => patchLine(i, { amount })}
                  style={styles.amount}
                />
              </View>
            ))}
            <Pressable testID={`${id}-add-line`} onPress={addLine} style={styles.addLine}>
              <Text style={styles.addLineText}>＋ Add line…</Text>
            </Pressable>
          </View>

          {/* live balance — CTA gated on 借方=貸方 */}
          <BalanceBar
            testID={`${id}-balance`}
            debits={debits}
            credits={credits}
            ctaLabel="Review balance →"
            onCta={() => setStep(2)}
          />
        </>
      ) : (
        <>
          {/* step 2 — review & post */}
          <View style={styles.reviewCard}>
            <View style={[styles.reviewRow, styles.reviewHead]}>
              <Text style={[styles.rcell, styles.rAcct, styles.muted]}>Account</Text>
              <Text style={[styles.rcell, styles.rSide, styles.muted]}>Type</Text>
              <Text style={[styles.rcell, styles.rAmt, styles.muted]}>This entry</Text>
            </View>
            {lines.map((line, i) => (
              <View key={line._key} testID={`${id}-review-${i}`} style={styles.reviewRow}>
                <Text style={[styles.rcell, styles.rAcct]}>{line.account || '—'}</Text>
                <View style={styles.rSide}>
                  <Badge
                    label={line.side === 'debit' ? '借方' : '貸方'}
                    status={line.side === 'debit' ? 'flagged' : 'posted'}
                  />
                </View>
                <Text
                  style={[
                    styles.rcell,
                    styles.rAmt,
                    styles.num,
                    { color: line.side === 'credit' ? color.credit : color.debit },
                  ]}
                >
                  {line.side === 'credit' ? '+ ' : '− '}
                  {formatGrouped(safeAmount(line.amount))}
                </Text>
              </View>
            ))}
          </View>

          <BalanceBar
            testID={`${id}-review-balance`}
            debits={debits}
            credits={credits}
            style={styles.reviewBalance}
          />

          <View style={styles.actions}>
            <Button label="← Back to entry" variant="ghost" onPress={() => setStep(1)} />
            <View style={styles.actionsRight}>
              <Button
                testID={`${id}-save-draft`}
                label="Save draft"
                variant="secondary"
                onPress={() => onSaveDraft?.(buildPayload())}
              />
              <Button
                testID={`${id}-post`}
                label="Post entry →"
                variant="primary"
                onPress={() => onSubmit?.(buildPayload())}
              />
            </View>
          </View>
        </>
      )}
    </View>
  );
}

function StepTab({
  label,
  active,
  onPress,
}: Readonly<{
  label: string;
  active: boolean;
  onPress?: () => void;
}>) {
  return (
    <Pressable onPress={onPress} disabled={!onPress}>
      <Text style={[styles.stepTab, active && styles.stepTabActive]}>{label}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  form: { flexDirection: 'column', gap: 16 },
  navrow: { flexDirection: 'row', gap: 18 },
  stepTab: { fontFamily: font.mono, fontSize: 12, color: color.ink3 },
  stepTabActive: { color: color.ink, borderBottomWidth: 2, borderBottomColor: color.ink, paddingBottom: 3 },
  alert: {},
  card: {
    backgroundColor: color.surface,
    borderWidth: 1,
    borderColor: color.hairline,
    borderRadius: radius.md,
    padding: 24,
  },
  eyebrow: {
    fontFamily: font.mono,
    fontSize: 10.5,
    letterSpacing: 1.8,
    textTransform: 'uppercase',
    color: color.ink3,
    marginBottom: 12,
  },
  sentence: { fontFamily: font.serif, fontSize: 21, lineHeight: 34, color: color.ink },
  captureFields: { flexDirection: 'row', gap: 12, marginTop: 16, flexWrap: 'wrap' },
  growWide: { flex: 2, minWidth: 200 },
  hl: { backgroundColor: withAlpha(color.highlight, 0.4), fontStyle: 'italic', color: color.ink },
  hlNum: { backgroundColor: withAlpha(color.highlight, 0.4), fontFamily: font.mono, color: color.ink },
  lines: {
    borderWidth: 1,
    borderColor: color.hairline,
    borderRadius: radius.md,
    backgroundColor: color.surface,
    padding: 12,
    gap: 12,
  },
  lineRow: { flexDirection: 'row', alignItems: 'flex-end', gap: 10, flexWrap: 'wrap' },
  grow: { flex: 1, minWidth: 160 },
  amount: { width: 140 },
  addLine: { paddingVertical: 6 },
  addLineText: { fontFamily: font.mono, fontSize: 13, color: color.ink3 },
  reviewCard: {
    borderWidth: 1,
    borderColor: color.hairline,
    borderRadius: radius.md,
    backgroundColor: color.surface,
    overflow: 'hidden',
  },
  reviewRow: { flexDirection: 'row', alignItems: 'center', gap: 12, paddingVertical: 8, paddingHorizontal: 12 },
  reviewHead: { backgroundColor: color.bgElev },
  rcell: { fontFamily: font.mono, fontSize: 12.5, color: color.ink },
  muted: { color: color.ink3, fontSize: 10.5, letterSpacing: 1, textTransform: 'uppercase' },
  rAcct: { flex: 2 },
  rSide: { flex: 1 },
  rAmt: { flex: 1.4, textAlign: 'right' },
  num: { textAlign: 'right', fontVariant: ['tabular-nums'] },
  reviewBalance: {},
  actions: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  actionsRight: { flexDirection: 'row', gap: 10 },
});
