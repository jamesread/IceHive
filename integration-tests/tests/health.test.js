/**
 * Controller integration tests require a reachable MySQL and these env vars:
 *   ICEHIVE_INTEGRATION_MYSQL_HOST, ICEHIVE_INTEGRATION_MYSQL_USER,
 *   ICEHIVE_INTEGRATION_MYSQL_DATABASE, optional ICEHIVE_INTEGRATION_MYSQL_PASSWORD,
 *   optional ICEHIVE_INTEGRATION_MYSQL_PORT (default 3306).
 * If unset, the suite is skipped.
 */
import { strict as assert } from 'node:assert';
import { spawn } from 'node:child_process';
import { setTimeout as sleep } from 'node:timers/promises';
import { mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { Builder } from 'selenium-webdriver';
import chrome from 'selenium-webdriver/chrome.js';

const CONTROLLER_PORT = 18080;
const CONTROLLER_URL = `http://127.0.0.1:${CONTROLLER_PORT}`;

let controllerProcess;
let driver;
let fixtureDir;

function mysqlEnv() {
  return (
    process.env.ICEHIVE_INTEGRATION_MYSQL_HOST &&
    process.env.ICEHIVE_INTEGRATION_MYSQL_USER &&
    process.env.ICEHIVE_INTEGRATION_MYSQL_DATABASE
  );
}

function writeFixture() {
  fixtureDir = join(tmpdir(), `icehive-controller-it-${Date.now()}`);
  mkdirSync(join(fixtureDir, 'migrations'), { recursive: true });

  const port = Number(process.env.ICEHIVE_INTEGRATION_MYSQL_PORT ?? 3306);
  const cfg =
    `listen: ${JSON.stringify(`:${CONTROLLER_PORT}`)}\n` +
    'mysql:\n' +
    `  host: ${JSON.stringify(process.env.ICEHIVE_INTEGRATION_MYSQL_HOST)}\n` +
    `  port: ${port}\n` +
    `  user: ${JSON.stringify(process.env.ICEHIVE_INTEGRATION_MYSQL_USER)}\n` +
    `  password: ${JSON.stringify(process.env.ICEHIVE_INTEGRATION_MYSQL_PASSWORD ?? '')}\n` +
    `  database: ${JSON.stringify(process.env.ICEHIVE_INTEGRATION_MYSQL_DATABASE)}\n`;
  writeFileSync(join(fixtureDir, 'config.yaml'), cfg, 'utf8');

  const up = `CREATE TABLE IF NOT EXISTS icehive_meta (
    k VARCHAR(255) NOT NULL PRIMARY KEY,
    v TEXT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`;
  const down = 'DROP TABLE IF EXISTS icehive_meta;\n';
  writeFileSync(join(fixtureDir, 'migrations', '000001_initial_schema.up.sql'), up, 'utf8');
  writeFileSync(join(fixtureDir, 'migrations', '000001_initial_schema.down.sql'), down, 'utf8');
}

async function waitForController(url, retries = 40, delay = 500) {
  for (let i = 0; i < retries; i++) {
    try {
      const res = await fetch(url);
      if (res.ok) return;
    } catch {
      // not ready yet
    }
    await sleep(delay);
  }
  throw new Error(`Controller at ${url} did not become ready`);
}

describe('Controller health', function () {
  this.timeout(120_000);

  before(function () {
    if (!mysqlEnv()) {
      this.skip();
    }
    writeFixture();
    controllerProcess = spawn(
      '../services/controller/controller',
      ['-configdir', fixtureDir],
      { cwd: fixtureDir, stdio: 'pipe' },
    );

    return waitForController(`${CONTROLLER_URL}/metrics`);
  });

  after(async function () {
    if (driver) {
      await driver.quit();
    }
    if (controllerProcess) {
      controllerProcess.kill('SIGTERM');
    }
    if (fixtureDir) {
      try {
        rmSync(fixtureDir, { recursive: true, force: true });
      } catch {
        // ignore cleanup errors
      }
    }
  });

  it('responds to /metrics', async function () {
    if (!mysqlEnv()) {
      this.skip();
    }
    const res = await fetch(`${CONTROLLER_URL}/metrics`);
    assert.equal(res.status, 200);
  });

  it('opens the frontend in a browser', async function () {
    if (!mysqlEnv()) {
      this.skip();
    }
    const options = new chrome.Options().addArguments('--headless', '--no-sandbox');
    driver = await new Builder().forBrowser('chrome').setChromeOptions(options).build();

    await driver.get(CONTROLLER_URL);
    const title = await driver.getTitle();
    assert.ok(title, 'page has a title');
  });
});
