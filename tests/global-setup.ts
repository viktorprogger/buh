import { execSync } from 'child_process';

// Creates (or resets) the Playwright test accountant inside the running dev container.
// Requires: docker compose up --profile dev
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
}
