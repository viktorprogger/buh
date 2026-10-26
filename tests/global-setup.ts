import { execSync } from 'child_process';

// Creates (or resets) the Playwright test accountant inside the running dev container.
// Requires: docker compose up --profile dev
const UPDATE_FIXTURE_ID = '00000000-0000-0000-0000-000000000044';
const KPO_FIXTURE_ENTREPRENEUR_ID = '00000000-0000-0000-0000-000000000045';
const KPO_FIXTURE_BOOK_ID = '00000000-0000-0000-0000-000000000046';

function seedUpdatesFixture() {
  const sql = `
INSERT INTO app_updates (id, title_sr, body_sr, title_en, body_en, title_ru, body_ru, published_at)
VALUES (
  '${UPDATE_FIXTURE_ID}',
  'Тестовое обновление', '<p>Проверка списка обновлений.</p>',
  'Test update', '<p>Checking the updates feed.</p>',
  'Тестовое обновление', '<p>Проверка списка обновлений.</p>',
  now()
)
ON CONFLICT (id) DO UPDATE SET
  title_sr = EXCLUDED.title_sr, body_sr = EXCLUDED.body_sr,
  title_en = EXCLUDED.title_en, body_en = EXCLUDED.body_en,
  title_ru = EXCLUDED.title_ru, body_ru = EXCLUDED.body_ru,
  published_at = EXCLUDED.published_at;
DELETE FROM app_update_reads WHERE update_id = '${UPDATE_FIXTURE_ID}';
UPDATE accountants SET language = 'sr' WHERE email = 'pw@test.local';
`;

  try {
    execSync('docker compose exec -T db psql -U buh -d buh -v ON_ERROR_STOP=1', {
      cwd: process.cwd(),
      input: sql,
      stdio: ['pipe', 'pipe', 'pipe'],
    });
    return;
  } catch {
    // The fallback is used when tests run against a locally managed PostgreSQL.
    const databaseURL = process.env.BUH_DATABASE_URL;
    if (!databaseURL) throw new Error('Could not seed the updates fixture database');
    execSync(`psql "${databaseURL}" -v ON_ERROR_STOP=1`, {
      cwd: process.cwd(),
      input: sql,
      stdio: ['pipe', 'pipe', 'pipe'],
    });
  }
}

function seedKPODisplayFixture() {
  // The applied row uses a cached rate; the pending row deliberately has no
  // cache entry. This keeps browser coverage independent of NBS availability.
  const sql = `
DO $$
DECLARE accountant_id UUID;
BEGIN
  SELECT id INTO accountant_id FROM accountants WHERE email = 'pw@test.local';
  INSERT INTO managed_entrepreneurs (id, accountant_id, name, pib)
  VALUES ('${KPO_FIXTURE_ENTREPRENEUR_ID}', accountant_id, 'Playwright KPO display fixture', '900000045')
  ON CONFLICT (id) DO UPDATE SET accountant_id = EXCLUDED.accountant_id, name = EXCLUDED.name, pib = EXCLUDED.pib;
  INSERT INTO kpo_books (id, managed_entrepreneur_id, year)
  VALUES ('${KPO_FIXTURE_BOOK_ID}', '${KPO_FIXTURE_ENTREPRENEUR_ID}', EXTRACT(YEAR FROM CURRENT_DATE)::int)
  ON CONFLICT (id) DO NOTHING;
  DELETE FROM kpo_entries WHERE kpo_book_id = '${KPO_FIXTURE_BOOK_ID}';
  INSERT INTO exchange_rates (requested_date, effective_date, currency, unit, middle_rate, fetched_at, source_url)
  VALUES (make_date(EXTRACT(YEAR FROM CURRENT_DATE)::int, 1, 15), make_date(EXTRACT(YEAR FROM CURRENT_DATE)::int, 1, 15), 'EUR', 1, 117.500000, now(), 'playwright-cache')
  ON CONFLICT (requested_date, currency) DO UPDATE SET middle_rate = EXCLUDED.middle_rate;
  INSERT INTO kpo_entries (kpo_book_id, collection_date, invoice_number, description, product_revenue, service_revenue, original_product_revenue, original_service_revenue, currency, rate_unit, rate_middle, rate_effective_date, rate_fetched_at, rate_source_url, rate_status, position)
  VALUES
    ('${KPO_FIXTURE_BOOK_ID}', make_date(EXTRACT(YEAR FROM CURRENT_DATE)::int, 1, 15), 'PW-EUR-1', 'Applied foreign row', 587500, 235000, 5000, 2000, 'EUR', 1, 117.5, make_date(EXTRACT(YEAR FROM CURRENT_DATE)::int, 1, 15), now(), 'playwright-cache', 'applied', 1),
    ('${KPO_FIXTURE_BOOK_ID}', make_date(EXTRACT(YEAR FROM CURRENT_DATE)::int, 1, 16), 'PW-RSD-1', 'RSD row', 300, 0, 300, 0, 'RSD', 1, 1, make_date(EXTRACT(YEAR FROM CURRENT_DATE)::int, 1, 16), now(), '', 'applied', 2),
    ('${KPO_FIXTURE_BOOK_ID}', make_date(EXTRACT(YEAR FROM CURRENT_DATE)::int, 1, 17), 'PW-PENDING-1', 'Pending foreign row', 0, 0, 1234.56, 0, 'EUR', 0, 0, make_date(EXTRACT(YEAR FROM CURRENT_DATE)::int, 1, 17), now(), '', 'pending', 3);
END $$;
`;
  try {
    execSync('docker compose exec -T db psql -U buh -d buh -v ON_ERROR_STOP=1', { cwd: process.cwd(), input: sql, stdio: ['pipe', 'pipe', 'pipe'] });
  } catch {
    const databaseURL = process.env.BUH_DATABASE_URL;
    if (!databaseURL) throw new Error('Could not seed the KPO display fixture database');
    execSync(`psql "${databaseURL}" -v ON_ERROR_STOP=1`, { cwd: process.cwd(), input: sql, stdio: ['pipe', 'pipe', 'pipe'] });
  }
}

export default async function globalSetup() {
  try {
    execSync(
      'docker compose exec -T app ./tmp/buh accountant-create --email pw@test.local --password pw-test-123 --restore',
      { cwd: process.cwd(), stdio: 'pipe' },
    );
  } catch {
    // Container might not be running — try building and running the binary directly.
    // This path is used when running tests with a local DB (BUH_DATABASE_URL in env).
    execSync(
      'go run . accountant-create --email pw@test.local --password pw-test-123 --restore',
      { cwd: process.cwd(), stdio: 'pipe' },
    );
  }
  seedUpdatesFixture();
  seedKPODisplayFixture();
}
