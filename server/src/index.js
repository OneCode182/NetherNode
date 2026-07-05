const fs = require('node:fs/promises');
const http = require('node:http');
const path = require('node:path');
const crypto = require('node:crypto');
const process = require('node:process');

const config = {
  port: Number(process.env.WORKER_HTTP_PORT || 8080),
  pollIntervalMs: Number(process.env.WORKER_POLL_INTERVAL_SECONDS || 5) * 1000,
  dryRun: String(process.env.WORKER_DRY_RUN || 'true').toLowerCase() === 'true',
  inboxDir: process.env.WORKER_WORKFLOW_INBOX || '/app/workflows/inbox',
  doneDir: process.env.WORKER_WORKFLOW_DONE || '/app/workflows/processed',
  failedDir: process.env.WORKER_WORKFLOW_FAILED || '/app/workflows/failed',
  obsPath: process.env.WORKER_OBS_PATH || '/app/observability',
  inboxMaxFiles: Number(process.env.WORKER_INBOX_MAX_FILES || 32),
};

const state = {
  received: 0,
  processed: 0,
  skipped: 0,
  failed: 0,
  lastProcessedAt: null,
  startedAt: Date.now(),
  runningScan: false,
};

async function safeMkdir(target) {
  await fs.mkdir(target, { recursive: true });
}

function writeJson(res, code, data, contentType = 'application/json') {
  const body = typeof data === 'string' ? data : JSON.stringify(data);
  res.statusCode = code;
  res.setHeader('content-type', contentType);
  res.setHeader('cache-control', 'no-store');
  res.end(body);
}

function sendError(res, code, message, extra = {}) {
  writeJson(res, code, {
    ok: false,
    message,
    ...extra,
  });
}

async function writeEvent(event) {
  const payload = {
    ts: new Date().toISOString(),
    worker: 'nethernode-worker',
    ...event,
  };
  await safeMkdir(config.obsPath);
  await fs.appendFile(path.join(config.obsPath, 'events.jsonl'), `${JSON.stringify(payload)}\n`);
}

function toMs(value, fallback) {
  const num = Number(value);
  return Number.isFinite(num) && num > 0 ? num : fallback;
}

async function archiveWorkflow(filePath, targetDir, result) {
  const fileName = path.basename(filePath);
  const resultPath = path.join(targetDir, `${fileName}.result.json`);
  await fs.copyFile(filePath, path.join(targetDir, fileName));
  await fs.writeFile(resultPath, `${JSON.stringify({ result, time: new Date().toISOString() })}\n`);
  await fs.unlink(filePath);
}

async function runWorkflow(payload) {
  if (!payload || typeof payload !== 'object') {
    return { status: 'failed', detail: 'payload must be an object' };
  }

  const type = payload.type || 'note';

  if (type === 'note') {
    await writeEvent({
      type: 'workflow.note',
      payload,
    });
    return { status: 'ok', detail: 'note captured', dryRun: config.dryRun };
  }

  if (type === 'echo') {
    const message = payload.message || '';
    await writeEvent({
      type: 'workflow.echo',
      message,
      dryRun: config.dryRun,
    });
    return { status: 'ok', detail: `echo: ${message}` };
  }

  return { status: 'skipped', detail: `unsupported workflow type ${type}` };
}

async function processFile(fileName) {
  const filePath = path.join(config.inboxDir, fileName);
  const contents = await fs.readFile(filePath, 'utf8');
  let payload;
  try {
    payload = JSON.parse(contents);
  } catch (error) {
    state.failed += 1;
    await archiveWorkflow(filePath, config.failedDir, { status: 'failed', detail: error.message });
    await writeEvent({ type: 'workflow.invalid-json', file: fileName, error: error.message });
    return;
  }

  try {
    const result = await runWorkflow(payload);
    state.processed += 1;
    state.lastProcessedAt = new Date().toISOString();
    if (result.status === 'skipped') {
      state.skipped += 1;
    }
    await archiveWorkflow(filePath, config.doneDir, result);
    await writeEvent({ type: 'workflow.processed', file: fileName, status: result.status });
  } catch (error) {
    state.failed += 1;
    await archiveWorkflow(filePath, config.failedDir, { status: 'failed', detail: error.message });
    await writeEvent({ type: 'workflow.failed', file: fileName, error: error.message });
  }
}

async function scanInbox() {
  if (state.runningScan) {
    return;
  }
  state.runningScan = true;
  try {
    const files = await fs.readdir(config.inboxDir);
    const pending = files
      .filter((fileName) => fileName.endsWith('.json'))
      .sort()
      .slice(0, config.inboxMaxFiles);
    for (const fileName of pending) {
      await processFile(fileName);
    }
  } catch (error) {
    if (error.code !== 'ENOENT') {
      await writeEvent({ type: 'scan.error', message: error.message });
    }
  } finally {
    state.runningScan = false;
  }
}

async function queueWorkflow(payload) {
  await safeMkdir(config.inboxDir);
  const id = crypto.randomUUID();
  const filePath = path.join(config.inboxDir, `${Date.now()}-${id}.json`);
  await fs.writeFile(filePath, `${JSON.stringify(payload)}\n`);
  state.received += 1;
  await writeEvent({ type: 'workflow.received', file: path.basename(filePath) });
  return { id, file: path.basename(filePath) };
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    req.on('data', (chunk) => chunks.push(chunk));
    req.on('error', reject);
    req.on('end', () => {
      if (!chunks.length) {
        resolve({});
        return;
      }
      try {
        resolve(JSON.parse(Buffer.concat(chunks).toString('utf8')));
      } catch (error) {
        reject(error);
      }
    });
  });
}

async function metrics() {
  return [
    '# HELP nethernode_worker_uptime_seconds Uptime in seconds.',
    `nethernode_worker_uptime_seconds ${Math.floor((Date.now() - state.startedAt) / 1000)}`,
    '# HELP nethernode_worker_received_total Workflows accepted.',
    `nethernode_worker_received_total ${state.received}`,
    '# HELP nethernode_worker_processed_total Workflows processed.',
    `nethernode_worker_processed_total ${state.processed}`,
    '# HELP nethernode_worker_skipped_total Workflows skipped.',
    `nethernode_worker_skipped_total ${state.skipped}`,
    '# HELP nethernode_worker_failed_total Workflows failed.',
    `nethernode_worker_failed_total ${state.failed}`,
    '# HELP nethernode_worker_dry_run Dry-run mode enabled.',
    `nethernode_worker_dry_run ${config.dryRun ? 1 : 0}`,
    '',
  ].join('\n');
}

async function startServer() {
  await safeMkdir(config.inboxDir);
  await safeMkdir(config.doneDir);
  await safeMkdir(config.failedDir);

  const server = http.createServer(async (req, res) => {
    const url = new URL(req.url || '/', `http://${req.headers.host}`);
    if (req.method === 'GET' && url.pathname === '/health') {
      writeJson(res, 200, {
        ok: true,
        status: 'running',
        dryRun: config.dryRun,
        startedAt: new Date(state.startedAt).toISOString(),
        uptimeSec: Math.floor((Date.now() - state.startedAt) / 1000),
        received: state.received,
        processed: state.processed,
        skipped: state.skipped,
        failed: state.failed,
        lastProcessedAt: state.lastProcessedAt,
      });
      return;
    }

    if (req.method === 'GET' && url.pathname === '/ready') {
      writeJson(res, 200, { ready: true });
      return;
    }

    if (req.method === 'GET' && url.pathname === '/metrics') {
      res.statusCode = 200;
      res.setHeader('content-type', 'text/plain; version=0.0.4');
      res.end(await metrics());
      return;
    }

    if (req.method === 'POST' && url.pathname === '/workflows') {
      try {
        const payload = await readBody(req);
        const accepted = await queueWorkflow(payload);
        writeJson(res, 202, { ok: true, ...accepted });
      } catch (error) {
        sendError(res, 400, 'invalid workflow payload', { error: error.message });
      }
      return;
    }

    sendError(res, 404, 'not found');
  });

  server.listen(toMs(config.port, 8080), () => {
    console.log(`nethernode-worker listening on :${config.port}`);
  });

  setInterval(scanInbox, toMs(config.pollIntervalMs, 5000));
  await writeEvent({ type: 'worker.started', port: config.port, dryRun: config.dryRun });

  process.on('SIGTERM', async () => {
    await writeEvent({ type: 'worker.shutdown' });
    server.close(() => process.exit(0));
  });

  return server;
}

startServer().catch((error) => {
  console.error(error);
  process.exit(1);
});

module.exports = {
  config,
  state,
  metrics,
  runWorkflow,
};
