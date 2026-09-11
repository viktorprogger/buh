# Security Bug Fixes Round 2 (review.json)

## Problem
Five bugs from review.json:
1. slip_merge.go: Accountant can delete any slip via `discard` field — no ownership check on loser slip
2. sliprecord.go: CopyToEntrepreneurUser has no ON CONFLICT; double-submit duplicates all slip records
3. slip.go: findOwnedSlip passes zero UUID to FindByID when slip is entrepreneur-user-owned → 404 instead of 403
4. importer.go: ProcessFileDataForEntrepreneurUser silently swallows upsert errors → returns (nil, nil) on DB failure
5. importer.go: tmp.Write(data) return ignored → disk-full causes misleading "no QR codes found"

## Approach

**Bug 1**: After parsing loserID, call FindByID and verify the loser slip belongs to this merge's entrepreneur or entrepreneur_user before deleting.

**Bug 2**: Add migration 039 with dedup + unique index on (entrepreneur_user_id, purpose, year, advance) WHERE entrepreneur_user_id IS NOT NULL. Then use ON CONFLICT DO NOTHING in CopyToEntrepreneurUser.

**Bug 3**: Before calling entrepreneurs.FindByID, check if s.ManagedEntrepreneurID == uuid.Nil (means entrepreneur-user-owned slip) and return 403.

**Bug 4**: Return upsertErr instead of continue on DB failure in ProcessFileDataForEntrepreneurUser.

**Bug 5**: Check tmp.Write return value and propagate error in both ProcessFileData and ProcessFileDataForEntrepreneurUser.

## Solution Implemented

All 5 fixes applied as described above.
