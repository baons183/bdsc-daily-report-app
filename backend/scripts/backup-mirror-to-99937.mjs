// Daily one-way incremental backup: copies every table of this app's own
// live database (DB_* in .env) into a dedicated BDSC_DailyReport database on
// 192.168.99.37 (BACKUP99937_* in .env), so there is always an up-to-date
// off-box copy even though this machine is the one actually serving the app.
//
// Table-by-table, insert-new/update-changed only (no delete/recreate): each
// table's source and target rows are compared by key, new rows are bulk-
// inserted, changed rows are bulk-staged then applied via a single UPDATE...
// FROM join, and rows already identical are left untouched. Rows removed
// from source are intentionally NOT removed from target - this is
// additive-only. Failure on one table is logged and does NOT stop the
// remaining tables from being processed.
//
// Adapted from checksheet-app's backend/scripts/backup-mirror-to-99937.mjs -
// same logic, different app/table list. Before this can run for the first
// time, the target database + a SQL login with rights on it must already
// exist on 192.168.99.37, and its schema must be provisioned once via
// `go run ./cmd/migrateremote -server=... -database=... -username=... -password=...`
// (reuses this app's own db.Migrate, so the remote schema is guaranteed
// identical to the local one).
//
// Connects with `database` pinned to BACKUP99937_DATABASE only - every query
// here is unqualified (dbo.[table]), so there is no way for this script to
// reach any other database on that SQL Server instance.
//
// One-way only: this only ever pushes local -> 99.37 - never run this the
// other direction by mistake.
import "dotenv/config";
import sql from "mssql";

// Listed parent-before-child (CongViec before CongViecAttachments, which has
// FK_CongViecAttachments_CongViec) so a first-time catch-up run that has to
// INSERT everything doesn't hit an FK violation. Every other table here is
// independent (see internal/db/db.go - only 1 FK exists in the whole schema).
const TABLES = [
  "Users",
  "SuCo", "GiamSat", "CongViec", "CongViecAttachments", "Reliability", "KPIEntries",
  "ReportRuns", "ReportApprovals", "CBMExclusions", "CBMInclusions",
  "ScheduleOverrides", "KPIBaseline", "ReportSchedule", "AppSettings"
];

// Every table above declares its own primary key, so no override is needed
// here today - kept as an extension point for a future table that doesn't.
const KEY_OVERRIDES = {};

function requireEnv(keys) {
  const missing = keys.filter((k) => !process.env[k]);
  if (missing.length) throw new Error("Thiếu biến môi trường trong .env: " + missing.join(", "));
}

function poolConfig(prefix) {
  return {
    server: process.env[`${prefix}_SERVER`],
    database: process.env[`${prefix}_DATABASE`],
    user: process.env[`${prefix}_USERNAME`],
    password: process.env[`${prefix}_PASSWORD`],
    port: process.env[`${prefix}_PORT`] ? Number(process.env[`${prefix}_PORT`]) : 1433,
    // Generous timeout as a safety margin against network hiccups to
    // 99.37 - every table synced here is small, so this is headroom, not a
    // requirement.
    requestTimeout: 300000,
    options: {
      encrypt: String(process.env[`${prefix}_ENCRYPT`] || "false").toLowerCase() === "true",
      trustServerCertificate: true
    }
  };
}

// result.recordset.columns gives back each column's type as a bare constructor
// (e.g. sql.NVarChar) without its length - reusing it as-is on the bulk Table
// silently defaults to length 1 and bcp then rejects any longer value ("Invalid
// column type from bcp client"). Re-attach the real length for the
// variable-length types; MAX columns report an odd/huge length depending on
// driver version, so anything <= 0 or absurdly large is treated as MAX.
const LENGTH_TYPES = new Set([sql.VarChar, sql.NVarChar, sql.Char, sql.NChar, sql.Binary, sql.VarBinary]);
function resolveColumnType(meta) {
  if (LENGTH_TYPES.has(meta.type)) {
    const len = meta.length;
    return meta.type(!len || len <= 0 || len > 4000 ? sql.MAX : len);
  }
  if (meta.type === sql.Decimal || meta.type === sql.Numeric) {
    return meta.type(meta.precision, meta.scale);
  }
  return meta.type;
}

// Auto-detects each table's real primary key from schema; falls back to
// KEY_OVERRIDES for the handful of tables that don't declare one.
async function getKeyColumns(conn, tables) {
  const result = await conn.request().query(`
    SELECT tc.TABLE_NAME, ku.COLUMN_NAME
    FROM INFORMATION_SCHEMA.TABLE_CONSTRAINTS tc
    JOIN INFORMATION_SCHEMA.KEY_COLUMN_USAGE ku ON tc.CONSTRAINT_NAME = ku.CONSTRAINT_NAME
    WHERE tc.CONSTRAINT_TYPE = 'PRIMARY KEY'
    ORDER BY ku.ORDINAL_POSITION
  `);
  const map = {};
  result.recordset.forEach((r) => {
    if (!tables.includes(r.TABLE_NAME)) return;
    (map[r.TABLE_NAME] = map[r.TABLE_NAME] || []).push(r.COLUMN_NAME);
  });
  for (const table of tables) {
    if (!map[table] || !map[table].length) {
      if (!KEY_OVERRIDES[table]) throw new Error(`Không tìm được khóa cho bảng ${table} - cần thêm vào KEY_OVERRIDES`);
      map[table] = KEY_OVERRIDES[table];
    }
  }
  return map;
}

async function fetchTable(conn, table) {
  const result = await conn.request().query(`SELECT * FROM dbo.[${table}]`);
  return { rows: result.recordset, columns: result.recordset.columns };
}

// null in any key column means the row can't be reliably matched to a
// counterpart on the other side - skip it entirely rather than guessing.
function buildKey(row, keyCols) {
  const parts = keyCols.map((c) => row[c]);
  if (parts.some((v) => v === null || v === undefined)) return null;
  return parts.map((v) => String(v)).join(" ");
}

// Dates round-trip through two separate driver connections as JS Date objects
// that can differ in representation despite meaning the same instant -
// compare by epoch ms rather than by reference/toString.
function normalizeValue(v) {
  if (v === null || v === undefined) return null;
  if (v instanceof Date) return v.getTime();
  return v;
}

function rowsEqual(a, b, colNames) {
  return colNames.every((c) => normalizeValue(a[c]) === normalizeValue(b[c]));
}

async function insertRows(target, table, rows, columns) {
  if (!rows.length) return;
  const colNames = Object.keys(columns);
  const bulkTable = new sql.Table(table);
  bulkTable.create = false;
  colNames.forEach((name) => {
    const meta = columns[name];
    bulkTable.columns.add(name, resolveColumnType(meta), { nullable: meta.nullable !== false, identity: !!meta.identity });
  });
  rows.forEach((row) => bulkTable.rows.add(...colNames.map((c) => row[c])));
  await target.request().bulk(bulkTable);
}

// Stages the changed rows into a temp table then applies them with a single
// UPDATE...FROM join, instead of one round trip per changed row. Runs inside
// a Transaction (a dedicated connection for its whole lifetime) rather than
// plain pool.request() calls, because #temp tables are connection-scoped -
// separate pool.request() calls can silently land on different pooled
// connections, which would make the temp table invisible to the UPDATE.
async function updateRows(pool, table, rows, columns, keyCols) {
  if (!rows.length) return;
  const colNames = Object.keys(columns);
  const nonKeyCols = colNames.filter((c) => !keyCols.includes(c));
  if (!nonKeyCols.length) return; // nothing but the key itself - never "changed"

  const stageName = `#Stage_${table}`;
  const transaction = new sql.Transaction(pool);
  await transaction.begin();
  try {
    const bulkTable = new sql.Table(stageName);
    bulkTable.create = true;
    colNames.forEach((name) => {
      const meta = columns[name];
      bulkTable.columns.add(name, resolveColumnType(meta), { nullable: true });
    });
    rows.forEach((row) => bulkTable.rows.add(...colNames.map((c) => row[c])));
    await new sql.Request(transaction).bulk(bulkTable);

    const setClause = nonKeyCols.map((c) => `t.[${c}] = s.[${c}]`).join(", ");
    const joinClause = keyCols.map((c) => `t.[${c}] = s.[${c}]`).join(" AND ");
    await new sql.Request(transaction).query(`
      UPDATE t SET ${setClause}
      FROM dbo.[${table}] t
      INNER JOIN ${stageName} s ON ${joinClause}
    `);

    await transaction.commit();
  } catch (err) {
    await transaction.rollback().catch(() => {});
    throw err;
  }
}

async function syncTable(source, target, table, keyCols) {
  const src = await fetchTable(source, table);
  const dst = await fetchTable(target, table);
  const colNames = Object.keys(src.columns);

  const targetMap = new Map();
  dst.rows.forEach((row) => {
    const key = buildKey(row, keyCols);
    if (key !== null) targetMap.set(key, row);
  });

  const toInsert = [];
  const toUpdate = [];
  let skipped = 0;

  src.rows.forEach((row) => {
    const key = buildKey(row, keyCols);
    if (key === null) {
      skipped++;
      return;
    }
    const existing = targetMap.get(key);
    if (!existing) toInsert.push(row);
    else if (!rowsEqual(row, existing, colNames)) toUpdate.push(row);
  });

  await insertRows(target, table, toInsert, src.columns);
  await updateRows(target, table, toUpdate, src.columns, keyCols);

  const unchanged = src.rows.length - toInsert.length - toUpdate.length - skipped;
  return { inserted: toInsert.length, updated: toUpdate.length, unchanged, skipped, total: src.rows.length };
}

async function main() {
  requireEnv([
    "DB_SERVER", "DB_DATABASE", "DB_USERNAME", "DB_PASSWORD",
    "BACKUP99937_SERVER", "BACKUP99937_DATABASE", "BACKUP99937_USERNAME", "BACKUP99937_PASSWORD"
  ]);

  const startedAt = Date.now();
  console.log(`[${new Date().toISOString()}] Bắt đầu backup BDSC_DailyReport -> 99.37 (incremental)...`);

  const source = await new sql.ConnectionPool(poolConfig("DB")).connect();
  const target = await new sql.ConnectionPool(poolConfig("BACKUP99937")).connect();

  let failures = 0;
  try {
    const keyColumnsByTable = await getKeyColumns(source, TABLES);

    for (const table of TABLES) {
      try {
        const r = await syncTable(source, target, table, keyColumnsByTable[table]);
        console.log(
          `  ${table}: +${r.inserted} mới, ~${r.updated} cập nhật, =${r.unchanged} không đổi` +
            (r.skipped ? `, bỏ qua ${r.skipped} dòng thiếu khóa` : "") +
            ` (tổng nguồn: ${r.total})`
        );
      } catch (err) {
        failures++;
        console.error(`  ${table}: LỖI - ${err.message}`);
      }
    }

    console.log(
      `[${new Date().toISOString()}] Hoàn tất trong ${Date.now() - startedAt}ms` +
        (failures ? ` (${failures} bảng lỗi, xem log ở trên)` : "")
    );
  } finally {
    await source.close();
    await target.close();
  }

  if (failures) process.exit(1);
}

main().catch((err) => {
  console.error(`[${new Date().toISOString()}] Backup THẤT BẠI:`, err.message);
  process.exit(1);
});
