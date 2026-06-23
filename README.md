# Sales API — Greenpark CEO War-Room

Backend Go untuk **Dashboard Sales Greenpark Group** (posisi menuju target 500 unit
2026). Data bersumber dari workbook `DASHBOARD SALES_GREENPARK_2026` (Q1 + Q2 2026).

Arsitektur berlapis (clean architecture), sama dengan layanan `finance` & `teknik`:

```
cmd/server         → composition root + graceful shutdown
internal/config    → konfigurasi via environment variable
internal/domain    → entitas inti (single source of truth bentuk data)
internal/repository→ boundary penyimpanan + in-memory store (seed data)
internal/service   → business logic + summary turunan
internal/transport → HTTP transport (router, handler, middleware)
```

## Menjalankan

```bash
cd backend/sales
go run ./cmd/server
# sales API listening on http://localhost:8085
```

Dengan **Google Sheets live-sync** (snapshot menarik DATA PENJUALAN + META ADS
INPUT langsung dari spreadsheet, refresh otomatis tiap interval):

```powershell
./run.ps1   # set kredensial + SALES_SYNC_INTERVAL_MIN, lalu go run ./cmd/server
```

Cek koneksi sheet tanpa menjalankan server (fetch + ingest, tanpa menulis data):

```powershell
$env:SALES_GOOGLE_CREDENTIALS = "...\keen-scion-...json"; go run ./cmd/checksync
```

### Konfigurasi (environment variable)

| Variabel                  | Default                | Keterangan                                                  |
| ------------------------- | ---------------------- | ----------------------------------------------------------- |
| `SALES_PORT`              | `8085`                 | Port HTTP                                                   |
| `SALES_ALLOW_ORIGIN`      | `*`                    | Origin CORS yang diizinkan                                  |
| `SALES_DATA_PATH`         | `data/sales-data.json` | File JSON tempat master data disimpan                       |
| `SALES_GOOGLE_CREDENTIALS`| `` (kosong → sync off) | Path service-account JSON (read-only Sheets). Kosong = sync nonaktif |
| `SALES_GSHEET_ID`         | `1FR0xlB5…D1j8`        | Spreadsheet sumber (DATA PENJUALAN, META ADS INPUT, dll.)   |
| `SALES_SYNC_INTERVAL_MIN` | `0` (manual)           | Interval auto-sync (menit); `0` = hanya sync manual         |

## Autentikasi

Semua endpoint (kecuali `GET /api/health` & `POST /api/auth/login`) butuh header
`Authorization: Bearer <token>`. Token didapat dari login dan berlaku 12 jam.

Akun default (ganti di produksi):

| Username | Password    | Role     | Akses                          |
| -------- | ----------- | -------- | ------------------------------ |
| `admin`  | `admin123`  | admin    | baca + tulis master data       |
| `viewer` | `viewer123` | viewer   | baca dashboard saja            |

## Endpoint

**Publik**

| Method | Path               | Keterangan                |
| ------ | ------------------ | ------------------------- |
| GET    | `/api/health`      | Health check              |
| POST   | `/api/auth/login`  | `{username,password}` → `{token,user}` |

**Terautentikasi (semua user)**

| Method | Path                   | Keterangan                              |
| ------ | ---------------------- | --------------------------------------- |
| GET    | `/api/auth/me`         | Profil sesi saat ini                    |
| POST   | `/api/auth/logout`     | Akhiri sesi                             |
| GET    | `/api/dashboard`       | Seluruh payload dashboard (1 panggilan) |
| GET    | `/api/summary`         | Ringkasan eksekutif turunan             |
| GET    | `/api/exec` `/api/funnel` `/api/projects` `/api/projects/{code}` `/api/channels` `/api/sales` `/api/reasons` `/api/agents` `/api/alerts` `/api/kpis` | Bagian-bagian dashboard |

**Admin only — master data (perubahan langsung tampil di dashboard)**

| Method | Path                       | Keterangan                            |
| ------ | -------------------------- | ------------------------------------- |
| PUT    | `/api/meta`                | Periode & tanggal update              |
| PUT    | `/api/exec`                | Executive snapshot                    |
| PUT    | `/api/stock`               | Master stock                          |
| PUT    | `/api/events`              | Event / walk-in                       |
| PUT    | `/api/funnel`              | Ganti seluruh tahap funnel            |
| PUT    | `/api/monthly`             | Ganti seluruh tren bulanan            |
| PUT    | `/api/reason-meta`         | Definisi layer reason code            |
| POST   | `/api/{projects\|sales\|channels\|reasons\|agents\|alerts\|kpis}`      | Buat (`_id` kosong) / update |
| DELETE | `/api/{projects\|sales\|channels\|reasons\|agents\|alerts\|kpis}/{id}` | Hapus by `_id`               |

Setiap baris koleksi punya `_id` sintetik (mis. `prj-verlim3`) sebagai kunci CRUD;
field bisnis (`code`, `name`, dst.) tetap bisa diubah bebas.

## Test

```bash
go test ./...
```

## Catatan

- Nilai uang pada API ini dalam **Rupiah penuh** (berbeda dengan layanan
  `finance` yang memakai satuan Rp juta).
- Data master persisten ke `SALES_DATA_PATH` (di-seed otomatis pada run pertama).
  Hapus file tersebut untuk mereset ke data seed. Jangan commit folder `data/`.
- Password di-hash salted SHA-256 (cukup untuk demo internal). Untuk produksi,
  ganti `internal/passwd` ke bcrypt/argon2.
