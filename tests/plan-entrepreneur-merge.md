# Test Plan: Entrepreneur Slips & Merge Flows

Tests to implement in two new spec files: `tests/entrepreneur_slips.spec.ts` and `tests/merge.spec.ts`.

Follows existing patterns: no direct DB access, all state via UI, `Date.now()` for unique identifiers.

---

## File 1: `tests/entrepreneur_slips.spec.ts`

### Setup

`beforeAll`: register entrepreneur user (unique email + password stored in closure).  
`beforeEach`: `loginEntrepreneur(page)`.

---

### Describe: "empty state"

**shows empty state message when no slips exist**
- navigate to `/e/slips`
- assert page contains empty state text (no table visible)

---

### Describe: "manual slip creation"

`beforeAll`: create one slip via `/e/slips/new` (store its URL).

**create slip — form saves and redirects to list**
- navigate to `/e/slips/new`
- fill: Purpose, Payee, PayeeAccount, PaymentCode=`253`, Amount=`5000`, Currency=`RSD`, Year=current year
- submit → redirected to `/e/slips`
- list contains a row with the purpose text

**create slip — appears under correct year header**
- navigate to `/e/slips`
- the year heading matching the submitted year is visible
- the slip's purpose is inside that section

**create slip — advance slip appears in advance subsection**
- create a second slip with "Advance" checkbox checked
- navigate to `/e/slips`
- the slip appears under the "Аванс" subheader, not in the main table

**create slip — previous year appears in collapsed section**
- create a slip with Year = current year − 1
- navigate to `/e/slips`
- a `<details>` element for that year exists (collapsed by default)
- expanding it reveals the slip

---

### Describe: "slip detail"

`beforeAll`: create one slip, save its URL.

**navigating to slip shows all saved field values**
- navigate to stored slip URL
- assert inputs match what was submitted (Purpose, Payee, Amount, PaymentCode, Year)

**edit and save persists changes**
- navigate to slip
- change Amount to a new value
- click "Сачувај"
- navigate to `/e/slips`, then back to slip URL
- Amount field shows new value

**download button triggers PDF download**
- navigate to slip
- set up download listener (`page.waitForEvent('download')`)
- click "Преузми PDF"
- assert download filename ends in `.pdf`

**delete removes slip from list**
- create a dedicated slip for this test (purpose unique per Date.now())
- navigate to its detail page
- set up dialog handler to accept
- click "Обриши"
- redirected to `/e/slips`
- list no longer contains the deleted slip's purpose

---

### Describe: "PDF upload — PIB missing"

`beforeAll`: register a fresh entrepreneur (unpaired, no PIB anywhere).  
`beforeEach`: `loginEntrepreneur` with that account.

**upload page shows PIB warning, no upload form**
- navigate to `/e/slips/upload`
- assert warning banner visible (contains "ПИБ" or "PIB")
- assert no `<form>` with file input is present

---

### Describe: "PDF upload — PIB present"

`beforeAll`:
1. `loginAccountant` — create entrepreneur with PIB set
2. Register entrepreneur user
3. Pair them (accountant sends invite; entrepreneur accepts)
4. `loginEntrepreneur`

**upload page shows file input when PIB is set**
- navigate to `/e/slips/upload`
- assert file input visible (no PIB warning)

**uploading valid PDF imports slips and shows results table**
- navigate to `/e/slips/upload`
- set file input to fixture PDF (`tests/fixtures/resenje.pdf`)
- submit form
- success banner is visible
- results table contains at least one row
- navigate to `/e/slips` — at least one slip exists in the list

**uploading non-PDF or empty file shows error**
- submit form with a `.txt` file
- error message visible on upload page

---

### Describe: "post-merge slip access"

*(depends on merge flow — see File 2; share the paired entrepreneur between suites or set up independently)*

`beforeAll`:
1. Accountant creates entrepreneur with PIB
2. Register entrepreneur user, pair them
3. Entrepreneur creates one slip (Year = current year)
4. Accountant triggers merge for that year (no conflicts → auto-merge)

**entrepreneur can still view slip after it moved to accountant's namespace**
- `loginEntrepreneur`
- navigate to `/e/slips`
- year section for current year is visible with "Спојено" badge
- the slip row is present and clickable
- detail page loads without error
- fields show correct values

**entrepreneur can edit a merged slip**
- navigate to merged slip detail
- change a field, save
- assert save succeeds (no error, no redirect to 403)

---

## File 2: `tests/merge.spec.ts`

Both KPO and slip merge follow the same pairing setup. Use one `beforeAll` for the whole file where possible.

### Shared Setup (top-level `beforeAll`)

1. `loginAccountant` — create entrepreneur (name + PIB unique per `Date.now()`)
2. Store entrepreneur page URL
3. Register entrepreneur user
4. Accountant sends invite via entrepreneur detail page
5. `loginEntrepreneur` — accept invite
6. Both users now paired

---

## Section A: KPO Merge

### Describe: "KPO auto-merge"

`beforeAll` (nested): Accountant's KPO book for current year is empty; entrepreneur adds 2 entries from their `/e/kpo` page.

**accountant triggers merge — no conflicts → redirect with success**
- `loginAccountant`
- navigate to entrepreneur detail page
- find and click "Споји КПО" (or merge link) for current year
- assert redirected to entrepreneur detail (no conflict page shown)
- success indicator (flash message or "Спојено" badge) visible

**after auto-merge, accountant KPO contains entrepreneur's entries**
- navigate to accountant KPO for that entrepreneur, current year
- assert the entries originally created by entrepreneur are present (match invoice numbers or amounts)

**after auto-merge, entrepreneur KPO shows merged badge**
- `loginEntrepreneur`
- navigate to `/e/kpo`
- current year shows "Спојено" badge (or equivalent merged indicator)

---

### Describe: "KPO manual merge — conflict: same ordinal, different data"

`beforeAll` (nested):
1. Accountant creates a KPO entry (e.g., invoice "2024-001", amount 10000)
2. Entrepreneur creates a KPO entry with same invoice number but different amount (e.g., 12000)

**merge view shows conflict card with both versions side by side**
- `loginAccountant`
- trigger merge for current year
- assert conflict page is shown (not auto-redirected)
- two columns visible: "Рачуновођа" and "Предузетник"
- both entries shown with differing amounts

**picking accountant version removes entrepreneur entry and continues**
- click "Задржи ову верзију" under accountant column
- redirected back to merge page (or to next conflict)
- entrepreneur's original entry no longer listed as conflict

**picking entrepreneur version moves entry to accountant book**
- (separate test with fresh setup)
- click "Задржи ову верзију" under entrepreneur column
- after finalize: accountant KPO contains the entrepreneur's version data

**finalize with resolved conflicts completes merge**
- after all conflicts resolved, click "Заврши спајање"
- redirected to entrepreneur detail with success indicator
- merged year badge visible

---

### Describe: "KPO merge — mixed: conflicts + auto-mergeable entries"

`beforeAll` (nested):
1. Accountant has entry A (no equivalent on entrepreneur side)
2. Entrepreneur has entry B (no equivalent on accountant side) → will auto-merge
3. Both have entry C with different amounts → conflict

**conflict page lists only entry C as conflict; entry B listed as auto-mergeable**
- trigger merge
- conflict section shows only entry C
- auto-merge preview section shows entry B

**after resolving conflict C and finalizing, both A and B are in accountant book**
- resolve C (pick either side)
- finalize
- navigate to accountant KPO — entries A, B (moved), and winning version of C all present

---

### Describe: "KPO merge — already merged year"

`beforeAll` (nested): Complete a merge for current year.

**merge button not shown (or disabled) for already-merged year**
- navigate to entrepreneur detail page
- assert no active "Споји КПО" button for the already-merged year

---

## Section B: Slip Merge

### Describe: "slip auto-merge — entrepreneur-only slips"

`beforeAll` (nested):
1. Entrepreneur creates 2 slips for current year (accountant has none)

**accountant triggers merge — auto-redirects, no conflict page**
- `loginAccountant`
- navigate to entrepreneur detail → click "Споји уплатнице" for current year
- assert redirected directly to entrepreneur detail (no `/merge/` URL in path)
- success indicator or "Спојено" badge visible

**after auto-merge, slips appear in accountant's list for that entrepreneur**
- navigate to accountant slip list for that entrepreneur (current year)
- both slips visible

**entrepreneur sees merged badge and can still access slips**
- `loginEntrepreneur` → `/e/slips`
- year section for current year shows "Спојено" badge
- both slip rows are present and open without error

---

### Describe: "slip auto-merge — identical slip on both sides"

`beforeAll` (nested):
1. Accountant creates slip: Purpose="Порез", Amount="5000", PayeeAccount="123-456-789", Year=current
2. Entrepreneur creates identical slip (same Purpose/Amount/PayeeAccount/Year/PaymentCode/Payer)

**merge auto-completes; accountant copy kept, entrepreneur copy deleted**
- trigger merge
- assert auto-redirect (no conflict page)
- accountant slip list shows exactly one slip with Purpose="Порез"
- after merge, no duplicate visible

---

### Describe: "slip manual merge — one conflict"

`beforeAll` (nested):
1. Accountant creates slip: Purpose="Порез", Amount="5000", Year=current
2. Entrepreneur creates slip: Purpose="Порез", Amount="6000", Year=current (same purpose/year/advance → conflict)

**merge view shows conflict with side-by-side comparison**
- trigger merge
- URL contains `/slips/merge/{year}`
- two columns show the two versions
- accountant version shows "5000"; entrepreneur version shows "6000"

**picking accountant version: entrepreneur slip deleted, merge continues**
- click "Задржи ову верзију" under accountant column
- assert no error; redirect to merge page
- no more conflicts on page (or finalize button now accessible)

**picking entrepreneur version: slip moved to accountant namespace**
- (separate beforeAll with same setup)
- click "Задржи ову верзију" under entrepreneur column
- finalize
- accountant slip list shows "6000" (entrepreneur's amount)

**POST /done while conflict still unresolved redirects back to merge page**
- manually POST to `/a/entrepreneurs/{id}/slips/merge/{year}/done` while conflict exists
- assert response is redirect back to the merge page, not to entrepreneur detail

---

### Describe: "slip manual merge — multiple conflicts"

`beforeAll` (nested):
1. Accountant: slips "Порез" 5000, "ПДВ" 2000
2. Entrepreneur: slips "Порез" 6000, "ПДВ" 2500 (both conflict)

**conflict page lists both conflicts**
- trigger merge
- two conflict cards visible (one per purpose)

**resolve first conflict, page reloads with only second conflict remaining**
- pick a version for "Порез"
- redirect back to merge page
- only one conflict card visible ("ПДВ")

**resolve second conflict, finalize succeeds**
- pick a version for "ПДВ"
- click "Заврши спајање"
- redirected to entrepreneur detail with merged badge

---

### Describe: "slip merge — mixed conflicts and auto-merge candidates"

`beforeAll` (nested):
1. Accountant: slip "Порез" 5000
2. Entrepreneur: slip "Порез" 6000 (conflict), slip "ПДВ" 2000 (entrepreneur-only, auto)

**conflict page shows "Порез" as conflict and "ПДВ" in auto-merge section**
- trigger merge
- conflict card for "Порез" visible
- auto-merge list contains "ПДВ"

**after resolving conflict and finalizing, both slips are in accountant namespace**
- resolve "Порез" conflict
- finalize
- accountant slip list contains both "Порез" (winning version) and "ПДВ" (moved from entrepreneur)

---

### Describe: "unpair — slips copied to entrepreneur"

`beforeAll` (nested):
1. Complete a merge for current year (accountant now owns the slips)
2. Accountant triggers unpair

**unpair copies merged slips to entrepreneur user**
- `loginEntrepreneur` → `/e/slips`
- year section visible, slips accessible
- no "Спојено" badge (merge records deleted)
- editing a slip succeeds

**after unpair, accountant's slip list for that entrepreneur is unchanged**
- `loginAccountant`
- navigate to entrepreneur slip list
- slips still present (copy was made, originals not deleted)

---

## Implementation Notes

- **Pairing flow**: needs a helper `pairEntrepreneur(accountantPage, entrepreneurPage, email)` covering invite-send → invite-accept steps. Extract this from the smoke test once it exists.
- **PDF fixture for entrepreneur upload**: reuse `tests/fixtures/resenje.pdf` if it contains a known PIB; otherwise add a dedicated fixture PDF.
- **Conflict setup shortcut**: there is no API seeding, so each conflict scenario must create slips via UI in `beforeAll`. Keep setups tight — one slip per side per test group.
- **Merge trigger location**: the merge button is on the accountant's entrepreneur detail page; confirm exact selector once implemented.
- **Year boundary**: all merge tests use `CURRENT_YEAR` (from helpers.ts) to avoid ambiguity.
