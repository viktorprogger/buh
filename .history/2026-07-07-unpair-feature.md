## Problem

Both accountant and entrepreneur need to be able to initiate unpair. During unpair, KPO books (all years) and slips must be fully copied from the managed entrepreneur side to the entrepreneur user's own account.

## Approach

1. Add `kpo.Repo.CopyToEntrepreneurUser` — copies books (ON CONFLICT DO NOTHING for years that already exist) and their entries via a single CTE query.
2. Update `handleUnpair` in `accountant/slip_merge.go` to call `kpoBooks.CopyToEntrepreneurUser` in addition to the existing slip copy.
3. Add `handleUnpair` in `entrepreneur/profile.go` — same copy+delete_merges+unpair flow, initiated by the entrepreneur.
4. Register `POST /unpair` route in `entrepreneur/handler.go`.
5. Add unpair button + confirmation dialog to accountant's `entrepreneur.html` and entrepreneur's `entrepreneur_profile.html`; add success banners (via `?unpaired=1` query param).
6. Add 6 i18n keys (2 per locale) for button label, confirm text, and success message.

## Key decisions

- **ON CONFLICT DO NOTHING** for KPO books copy: if the entrepreneur already has their own book for a year, it's preserved rather than overwritten. This avoids duplicating entries for users who added KPO manually.
- **Both sides keep records**: the accountant's managed_entrepreneur records are never touched; the entrepreneur receives a copy.
- **Entrepreneur-side `handleUnpair` lives in `profile.go`**: it's logically related to the profile (pairing status is shown there) and keeps the new file count low.
