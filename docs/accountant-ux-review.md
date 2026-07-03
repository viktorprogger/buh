# Accountant UX Review — Паушалне уплатнице

Review performed by: simulated Serbian accountant session (Playwright, July 2026)
Account used: `test@test.test` (Рачуновођа role)

---

## 🔴 Critical

### 1. No warning when паушалац exceeds the income limit
The test entrepreneur (Nemanja) has income of 27,876,690 RSD against a 6,000,000 RSD ceiling — 464.61% over limit. The app displays the percentage but raises no alert. When a паушалац exceeds the annual limit they are legally required to switch tax regimes. This is a compliance failure that the app silently ignores.

**Expected:** a prominent warning (banner, badge, email) as soon as income crosses the threshold, with guidance on next steps.

### 2. All contribution amounts show 0.00 RSD
Six payment slips are auto-generated (ПИО, ЗДРО, НЗС for 2026 and 2027) but every amount is 0.00 RSD. The landing page says "upload PDFs from e-Порез to auto-detect amounts," but it is not clear whether uploading actually populates these fields — and no feedback is given after upload. An accountant cannot pay anything in this state.

**Expected:** amounts populated from the uploaded e-Порез решење, with a clear indication of which решење each slip was derived from.

### 3. No KPO book export
КПО (Knjiga poslovnih prihoda — the official income ledger) is a legal document that must be printed and archived. There is no "Export to PDF" or "Print" button on the KPO table.

**Expected:** a print/PDF export action on the KPO section, producing an output that matches the official form layout required by Serbian tax law.

---

## 🟡 Significant gaps

### 4. Entrepreneur list has no status indicators
The list shows only name and PIB. With 10+ clients an accountant cannot see at a glance who is approaching the income limit, who has overdue contributions, or whose next payment deadline is coming up.

**Expected:** status column or color-coded badges showing limit usage %, next due date, and any overdue items.

### 5. New entrepreneur form asks for only two fields
Creating a new entrepreneur requires only Name and PIB. But to generate payment slips or any official document, МБ (registration number), address, and bank account are also needed — and can only be added in a second edit step.

**Expected:** the creation form should include all fields needed to make the entrepreneur immediately usable, or at minimum prompt the user to complete them before they can generate slips.

### 6. Mixed scripts in payment slip purposes
The UI is entirely in Cyrillic, but auto-generated payment slip descriptions use Latin script ("Doprinos za NZS za 2027. godinu"). Inconsistent and looks unfinished.

**Expected:** consistent use of one script throughout — either Cyrillic everywhere, or at least make the Latin-script field values user-editable.

### 7. No search or filter on entrepreneur list
No search input or filter on the entrepreneurs list. This will become unusable past ~15 clients.

**Expected:** a search field that filters by name or PIB.

---

## 🟠 Confusing behavior

### 8. "Укњижи" button has no explanation or confirmation
The "Book" button appears below the KPO table without any tooltip, confirmation dialog, or description of what it finalizes. It is unclear what state it moves data into, whether it is reversible, and what happens to the KPO entries afterwards.

**Expected:** a confirmation dialog explaining what booking does and warning that it may lock entries, or at minimum a tooltip.

### 9. "Аванс" checkbox on payment slip is unclear
The new payment slip form has an "Аванс" checkbox with a small "?" marker. Advance payments for паушалци are a specific legal concept. It is not evident what this checkbox changes in the generated slip or who should use it.

**Expected:** an inline explanation (tooltip or help text) describing when advance payment applies and how it affects the slip fields.

### 10. No history of uploaded e-Порез решења
The main dashboard has a "Учитај PDF решење" upload section, but there is no log of what was uploaded and when. After submitting PDFs, the accountant has no way to confirm the import succeeded or to audit which решење each payment amount came from.

**Expected:** an import history section (date, file name, entrepreneur matched, amounts extracted) accessible from the dashboard or the entrepreneur's page.

---

## 🔵 Nice-to-have / future work

- **Payment deadlines** — contributions are due by the 15th of each month; no calendar or "next due" indicator is shown anywhere.
- **ЈМБГ field** — the personal ID number is required on some official Serbian tax documents but is not stored.
- **Bulk slip generation** — no way to generate payment slips for all clients at once; each entrepreneur must be opened individually.
- **Delete entrepreneur** — no delete action visible on the list or detail page; unclear how to remove a client.
