# Accountant UX Review — Round 2

**Date:** 2026-07-03  
**Tested with:** Playwright headless Firefox (no real session)  
**Service:** http://localhost:8081  
**Login:** test@test.test / test (accountant role)

---

## Verification Results

### 1. Clicking a slip in an entrepreneur's profile opens the slip (not 404)
**PASS.** Each slip row has `onclick="window.location.href='/a/slips/{id}'"` and navigating to that URL returns a proper "Уплатница — Паушалне уплатнице" page (HTTP 200).

---

### 2. "Сачувај" and "Преузми PDF" buttons on a slip page work (not 404)
**PARTIAL PASS / FAIL.**

- **"Сачувај"** — PASS. Uses `formaction="/a/slips/{id}/save"`, submits via POST, receives HTTP 302 redirect back to the slip page. No errors.
- **"Преузми PDF"** — FAIL. Uses `formaction="/a/slips/{id}/download"`, submits via POST, receives HTTP 200 — but the response is `text/html` (the slip page itself), with no `Content-Disposition` header and no PDF binary. The download does not trigger. PDF generation is broken.

---

### 3. Colored dots in entrepreneur list have hover tooltips with percentage
**PASS.** Each dot renders as `<span ... title="Прекорачен паушални лимит (466%)">` and `title="Прекорачен ПДВ лимит (350%)"`. The native browser tooltip shows the percentage on hover.

---

### 4. Warning banner for exceeded paušal threshold says "Обавестите клијента"
**PASS.** The banner text is: *"КРИТИЧНО! Достигли сте паушални праг (466.3% — …). Хитно **обавестите клијента**."* The old wording ("консултујте са вашим књиговођом") is gone.

---

### 5. Entrepreneur detail page has "← Предузетници" breadcrumb at top
**PASS.** The detail page renders `<a href="/a/" class="hover:text-primary">← Предузетници</a>` immediately below the nav bar, above the entrepreneur card.

---

### 6. "Укњижи" button shows a confirmation dialog
**PASS.** The button sits inside `<form method="POST" action="…/kpo/2026/finalize" onsubmit="return confirm('Финализовати КПО за 2026 годину? Ова акција закључава књигу. Накнадне измене захтевају откључавање.')">`. The `onsubmit` confirm fires before submission.

---

### 7. New entrepreneur form has "Назив (кратко)" and "Жиро рачун" labels
**PASS.** The `/a/entrepreneurs/new` form contains `<label for="title">Назив (кратко)</label>` and `<label for="bank_account">Жиро рачун</label>`. Neither "Приказни назив" nor "Текући рачун" appears anywhere.

---

### 8. KPO amounts show with thousand separators
**PASS.** Non-zero amounts in the KPO table use the Serbian convention (narrow-space thousand separator): e.g. `13 213 120`, `2 342 340`, `12 321 230`, `100 000`. Raw unformatted integers are not shown.

---

## Summary

| # | Check | Result |
|---|-------|--------|
| 1 | Slip click opens slip | PASS |
| 2 | "Сачувај" works | PASS |
| 2 | "Преузми PDF" works | **FAIL** — returns HTML, not PDF |
| 3 | Dot tooltips with percentage | PASS |
| 4 | Banner says "Обавестите клијента" | PASS |
| 5 | "← Предузетници" breadcrumb | PASS |
| 6 | "Укњижи" shows confirm dialog | PASS |
| 7 | Correct form labels | PASS |
| 8 | KPO thousand separators | PASS |

7 of 9 checks pass. One functional regression/bug remains.

---

## Remaining Issues

### Critical
- **"Преузми PDF" is non-functional.** The `/a/slips/{id}/download` endpoint returns HTTP 200 with `Content-Type: text/html` — it renders the slip page HTML instead of generating a PDF. There is no `Content-Disposition: attachment` header and no binary is served. The user clicking the button gets no feedback and no file. This is the only unresolved bug from the original review list.

### Minor / New Observations
- The slip detail page (`/a/slips/{id}`) shows "← Назад на предузетника" but no link back to the full entrepreneur list. Two clicks are needed to reach the list from a slip. This is functional but slightly inconvenient — consider adding "← Предузетници" at the end or a breadcrumb trail.
- The KPO УКУПНО (total) row is rendered inline with JavaScript rather than in a `<tfoot>` element, which means screen-reader users and print stylesheets may not recognize it as a table footer.
- The "Обриши" (delete) button on the slip page submits the same form as "Сачувај" and "Преузми PDF" without a `type="submit" formaction="…/delete"` distinction that is visually separate — risk of accidental deletion is somewhat high since the red button sits on the same form.
