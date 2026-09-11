# Merge Settings Page into Entrepreneur Detail

**Date:** 2026-07-02

## Problem

The `/a/entrepreneurs/<id>/settings` page only held Address and BankAccount editing, creating a two-page UX for what is effectively one entity's profile. The requirement was also to verify that all KPO-required data is present.

## KPO Data Check

All required fields already exist in the `Entrepreneur` entity: Name, PIB, Address, BankAccount. The `kpo.Entry` struct has CollectionDate, InvoiceNumber, ProductRevenue, ServiceRevenue. No new fields needed.

## Approach

- Add Address and BankAccount to the view and edit sections of `entrepreneur.html`
- Update `handleEntrepreneurUpdate` to also save `address` and `bank_account` from form
- Remove `handleSettings` and `handleSettingsUpdate` from `accountant/entrepreneur.go`
- Remove the two settings routes from `accountant/handler.go`
- Remove `Settings` from the `shared.Templates` struct and `ParseTemplates`
- Keep `settings.html` file (no delete) — orphaned but harmless

## Outcome

Single page for all entrepreneur data. No redirect to /settings needed anywhere.
