import { execSync } from 'child_process';

// Creates (or resets) the Playwright test accountant inside the running dev container.
// Requires: docker compose up --profile dev
const UPDATE_FIXTURE_ID = '00000000-0000-0000-0000-000000000044';

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
}
