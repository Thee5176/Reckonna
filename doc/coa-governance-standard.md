# Chart of Accounts — Governance & Management Standard

| | |
|---|---|
| **Document** | COA Governance & Management Standard |
| **Applies to** | ERP general ledger, all entities and reporting frameworks |
| **Status** | Draft v1.0 |
| **Owner** | COA Steward (see §11) |
| **Last reviewed** | _set on adoption_ |
| **Review cadence** | Quarterly light review; annual full review (§15) |

---

## 1. Purpose and scope

This standard defines how the Chart of Accounts (COA) is structured, created, changed, and retired. Its primary goal is to make the COA **stable** — so that day-to-day transaction entry stays simple — while remaining **flexible enough** to support new business use cases and multiple reporting frameworks (e.g. IFRS, TFRS, US GAAP) without redesign.

It governs four distinct layers, which must never be conflated:

1. **Accounts** — the nature of an economic item.
2. **Dimensions** — the context of a transaction (entity, segment, project, etc.).
3. **Books (ledgers)** — framework-specific treatment of the same event.
4. **Mappings** — how accounts are presented in each framework's statements.

The most common and most damaging COA failures come from solving a problem in the wrong layer. §3 exists to prevent that.

---

## 2. Design principles

These four principles are binding. Every rule in this document derives from them.

**P1 — The account encodes nature only.**
An account answers "*what kind* of economic item is this?" (cash, trade receivable, rent expense). It must not encode who, where, which project, which currency, or which framework. The account list is the slowest-changing part of the system.

**P2 — Context lives in dimensions.**
Entity, segment, cost centre, project, counterparty, and currency are fields on the journal line, not part of the account code. Adding a new context is a data operation, not a COA change.

**P3 — Treatment differences live in books, not accounts.**
When two frameworks treat the *same* event differently (e.g. a lease capitalised under IFRS 16 but expensed under a local GAAP), the difference is recorded as a delta entry in a framework-specific book — never as a separate account.

**P4 — Presentation differences live in the mapping layer.**
The same account can roll up to different statement line items under different frameworks. Switching framework swaps the mapping, not the underlying data.

---

## 3. Decision framework for new use cases

**This is the core of the standard.** When a new business need appears, do not create an account by reflex. Work through the questions in order and stop at the first that applies.

> **Q1 — Is this a genuinely new *nature* of economic item that no existing account represents?**
> Example: the platform begins holding buyer funds in escrow — a liability the COA has never tracked.
> → **Create a new account** (§5–§10). This is the *only* trigger for a new account.

> **Q2 — Is it the same nature of item, just in a new *context*?**
> Example: expansion into a second country; a new project; a new operating segment; a new transaction currency.
> → **Add a dimension value** (§7). Do **not** create accounts.

> **Q3 — Is it the same economic event, but a framework treats it *differently*?**
> Example: development costs capitalised under IAS 38 but expensed under US GAAP.
> → **Post a delta in the relevant book** (§8). Do **not** create framework-specific accounts.

> **Q4 — Is the data already captured correctly, but a framework *presents* it differently?**
> Example: an item shown as "Other income" under one framework and netted against an expense under another.
> → **Update the mapping layer** (§9). Do **not** create accounts.

> **Q5 — Is this an entirely new reporting framework?**
> Example: an investor now requires US GAAP statements.
> → **Add a book + a mapping set** (§8, §9), then handle individual divergences via Q3.

If the answer to Q1 is "no", a new account is almost certainly the wrong solution. When in doubt, prefer dimensions and mappings over new accounts — they are reversible and do not pollute the account list.

---

## 4. Account code structure

Codes are 5 digits, assigned within reserved ranges. **Leave gaps** (increment new accounts by 10, sub-blocks by 100) so related accounts can be inserted in logical order later.

| Range | Class | Normal balance | Notes |
|---|---|---|---|
| 10000–13999 | Current assets | Debit | Cash, receivables, inventory, prepayments |
| 14000–19999 | Non-current assets | Debit | PP&E, intangibles, investments, deferred tax asset |
| 20000–23999 | Current liabilities | Credit | Payables, accruals, current lease liability, escrow |
| 24000–29999 | Non-current liabilities | Credit | Long-term debt, non-current lease liability |
| 30000–39999 | Equity | Credit | Capital, retained earnings, reserves, OCI reserves |
| 40000–49999 | Income / revenue | Credit | Contract revenue, fee income |
| 50000–59999 | Cost of sales / direct costs | Debit | Directly attributable costs |
| 60000–69999 | Operating expenses | Debit | Admin, marketing, staff, depreciation |
| 70000–79999 | Other income & finance items | Mixed | Interest, FX gains/losses, gains on disposal |
| 80000–89999 | Income tax | Mixed | Current and deferred tax |
| 90000–99999 | OCI / statistical / suspense | Mixed | Non-P&L items; suspense cleared monthly |

Rules:

- **R4.1** A code, once assigned, is permanent for the life of that account's *nature*. A retired code is never reused for a different purpose (§10).
- **R4.2** Postings are made only to detail (leaf) accounts. Header/summary accounts exist for roll-up only and must be flagged `postable = false`.
- **R4.3** The current vs non-current split for assets and liabilities is carried as an attribute (§6), not inferred from the range alone, so an item can be reclassified at period end without changing accounts.

---

## 5. Naming conventions

- **R5.1** Names describe nature in plain, framework-neutral language: `Trade receivables`, not `IFRS debtors` or `AsiaProjectReceivables`.
- **R5.2** Sentence case, no abbreviations except widely understood ones (VAT, PP&E).
- **R5.3** No context words in the name (no entity, project, region, or currency). If you are tempted to add one, the answer is a dimension (Q2).
- **R5.4** Names are unique within their class. Two accounts must never share a name.

---

## 6. Required account attributes

Every account carries this metadata. Accounts may not be activated until all required fields are set.

| Attribute | Required | Purpose |
|---|---|---|
| `code` | Yes | 5-digit identifier within reserved range |
| `name` | Yes | Plain-language nature |
| `description` | Yes | One sentence: what posts here, what does not |
| `type` | Yes | asset / liability / equity / income / expense |
| `normal_balance` | Yes | debit / credit |
| `postable` | Yes | false for header/summary accounts |
| `current_noncurrent` | Assets & liabilities | Drives statement classification |
| `ifrs_line_item` | Yes | Maps to IAS 1 / framework caption |
| `gaap_line_item` | When GAAP book active | Maps to GAAP caption |
| `measurement_category` | Financial instruments | amortised cost / FVOCI / FVTPL (IFRS 9) |
| `allowed_books` | Yes | Which books may post to this account |
| `required_dimensions` | Optional | e.g. counterparty mandatory for receivables |
| `status` | Yes | active / inactive / archived |
| `owner` | Yes | Accountable person/role |

---

## 7. Dimension governance

- **R7.1** Dimensions are the default answer to "we need to track X by Y". Reach for a dimension before an account.
- **R7.2** Each dimension is defined once as a *type* (e.g. `segment`, `project`, `counterparty`, `currency`); its members are data and can be added freely without COA change control.
- **R7.3** Dimensions must be orthogonal — do not create a `project` dimension whose values secretly encode entity. Keep each axis independent.
- **R7.4** An account may *require* specific dimensions (R6 `required_dimensions`) so that, for example, every receivable posting carries a counterparty. This is preferable to splitting the receivable into many accounts.
- **R7.5** Daily/personal entry uses dimension defaults (single entity, home currency, no segment) so the user never sees this complexity.

---

## 8. Book (ledger) governance

Books implement multi-framework support without duplicating accounts.

- **R8.1** The `base` book holds all framework-neutral activity — the large majority of transactions. Personal-scale use typically touches only this book.
- **R8.2** Each additional framework has a **delta book** (e.g. `ifrs`, `gaap`, `tfrs`) that records *only the differences* from base, never the full transaction.
- **R8.3** A framework's financial statements are computed as `base + that framework's delta book`.
- **R8.4** Each book must independently balance per journal entry (debits = credits). A delta adjustment is always self-balancing and never touches the base book.
- **R8.5** Do not create a delta book until a real divergence exists. Adding a book later is additive, not a migration.
- **R8.6** Treatment divergences that require a delta entry must cite the standard that causes them (e.g. "IAS 38 §57 capitalisation") in the entry description, for audit traceability.

---

## 9. Mapping layer governance

- **R9.1** Mappings are pure metadata linking each account to a statement and line item, per framework. They carry no transaction data.
- **R9.2** Every active account must have a complete mapping for every active framework before period close.
- **R9.3** Changing a mapping is reversible and does not alter posted data; it is therefore lower-risk than account changes, but still follows change control (§11) because it affects reported figures.
- **R9.4** A new framework (Q5) is onboarded by cloning the closest existing mapping set and adjusting, then adding a delta book for divergences.

---

## 10. Account lifecycle

**Create** — follow §3 (must pass Q1), assign a code with gaps (§4), set all required attributes (§6), obtain approval (§11), then activate.

**Modify**
- **R10.1** Renaming is permitted **only** if the account's nature is unchanged (e.g. clarifying wording). Record the reason.
- **R10.2** Changing an account's `type` or `normal_balance` is **prohibited** — it corrupts historical postings. Instead, create a new account and migrate via a documented reclassification entry.
- **R10.3** Mapping changes follow §9.

**Deprecate / archive**
- **R10.4** Accounts are **never deleted** — deletion orphans historical journal lines.
- **R10.5** To retire an account, set `status = inactive` (blocks new postings, retains history). After the account has a zero balance and a full reporting period has closed, it may be set to `archived`.
- **R10.6** A retired code is never reused for a different nature (R4.1).

---

## 11. Change control and roles

Scale the formality to context; the roles can be held by one person in a personal/small setup but should be named distinctly so the process scales.

| Role | Responsibility |
|---|---|
| **Requester** | Raises a change request with business justification and the §3 decision trail |
| **COA Steward (Owner)** | Guardian of this standard; reviews against §3, approves or rejects, assigns codes |
| **Reviewer** | For higher-risk changes (new framework, type reclassification), provides a second check |

Process:

1. Requester completes the checklist in §12.
2. Steward confirms the change is in the correct layer (the §3 questions are answered and recorded).
3. Steward approves; code assigned (for new accounts) or mapping/book updated.
4. Change is logged with date, requester, approver, and the decision trail.

Changes that **always** require a Reviewer in addition to the Steward: adding a reporting framework, reclassifying an account's type, or retiring an account with prior-period balances.

---

## 12. New account request checklist

Attach to every request for a new account. A request that cannot answer "yes" to item 1 should be redirected to a dimension, book, or mapping change.

- [ ] **1. Q1 confirmed:** This is a genuinely new *nature* of item, and Q2–Q4 have been ruled out (record why).
- [ ] **2. No existing account** already covers this nature (searched by name and description).
- [ ] **3. Class and range** selected per §4, with a gapped code proposed.
- [ ] **4. Name** complies with §5 (nature only, no context words).
- [ ] **5. All required attributes** (§6) specified, including `current_noncurrent` and `ifrs_line_item`.
- [ ] **6. Books**: `allowed_books` listed; any framework divergence noted for §8 handling.
- [ ] **7. Mappings**: line item identified for every active framework (§9).
- [ ] **8. Owner** assigned.

---

## 13. Prohibited patterns

These are the recurring mistakes this standard exists to prevent.

- **Encoding context into codes** — e.g. `40010-TH-ProjX`. Use dimensions (P2).
- **Framework-specific accounts** — e.g. separate `IFRS lease asset` and `GAAP lease asset`. Use a delta book (P3).
- **Duplicate accounts** for the same nature created because the existing one wasn't found. Search first (§12.2).
- **Deleting accounts** or reusing retired codes (R10.4, R10.6).
- **Posting to header/summary accounts** (R4.2).
- **Reclassifying `type` in place** instead of creating a new account (R10.2).
- **Catch-all "Miscellaneous" accounts** that accumulate untraceable activity. Create a properly-named account or a dimension instead.
- **Solving a presentation need with a new account** when a mapping change suffices (P4).

---

## 14. Worked examples

**A. Platform begins holding buyer funds in escrow before releasing to the seller.**
Q1 = yes — a liability not previously tracked. → New account, e.g. `21500 Customer escrow payable` (current liability, credit), with `counterparty` as a required dimension. No book or mapping novelty.

**B. Expansion to a second country.**
Q1 = no; Q2 = yes (new context). → Add an `entity` (and possibly `currency`) dimension value. **No new accounts.**

**C. An investor requires US GAAP statements.**
Q5 = yes. → Add a `gaap` book and clone the IFRS mapping set into a GAAP set, then handle individual divergences via Q3. **No new accounts.**

**D. Platform development costs: capitalised under IAS 38, expensed under US GAAP.**
Q3 = yes (treatment differs). → Record the cost in `base` as an expense; post an `ifrs` delta that capitalises it (Dr intangible asset, Cr the expense). IFRS statements (`base + ifrs`) show an asset; GAAP statements (`base` alone) show an expense. **No framework-specific accounts.**

**E. Revenue must be disaggregated by product category for IFRS 15 disclosure.**
Q2 = yes (same nature, new context). → Add a `product_category` dimension and disaggregate via reporting. **No new revenue accounts per category.**

---

## 15. Review and maintenance cadence

- **Quarterly (light):** review accounts created or retired in the quarter; confirm no prohibited patterns crept in; check suspense (90000-range) is cleared.
- **Annually (full):** review the entire COA against this standard ahead of financial-statement preparation; verify every active account has complete mappings for every active framework; archive eligible inactive accounts.
- **On framework change:** trigger an out-of-cycle review of mappings and delta books.

---

## Appendix — starter numbering ranges

A minimal personal/early-stage set (extend within the §4 ranges, leaving gaps):

| Code | Account | Type | Current/NC |
|---|---|---|---|
| 10000 | Cash and cash equivalents | Asset | Current |
| 11000 | Trade and other receivables | Asset | Current |
| 12000 | Prepayments | Asset | Current |
| 14000 | Property, plant and equipment | Asset | Non-current |
| 14500 | Intangible assets | Asset | Non-current |
| 20000 | Trade and other payables | Liability | Current |
| 21000 | Accrued expenses | Liability | Current |
| 21500 | Customer escrow payable | Liability | Current |
| 24000 | Lease liabilities | Liability | Non-current |
| 30000 | Contributed capital | Equity | — |
| 31000 | Retained earnings | Equity | — |
| 40000 | Service / fee revenue | Income | — |
| 50000 | Cost of services | Expense | — |
| 60000 | Staff costs | Expense | — |
| 61000 | Marketing and advertising | Expense | — |
| 62000 | General and administrative | Expense | — |
| 63000 | Depreciation and amortisation | Expense | — |
| 70000 | Finance income | Income | — |
| 71000 | Finance costs | Expense | — |
| 80000 | Income tax expense | Expense | — |

---

*End of standard. Changes to this document itself follow the same change-control process in §11.*
