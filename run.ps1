# Launch the Sales (penjualan) dashboard API with Google Sheets live-sync ON.
#
# Wires the snapshot to the live spreadsheet: the engine fetches DATA PENJUALAN
# + META ADS INPUT (and the master tabs) on startup and every SyncIntervalMin
# minutes, so the Executive Sales Snapshot tracks the sheet automatically.
#
# Env vars can be overridden before calling; the defaults below are the wired
# production values. Run:  ./run.ps1
$ErrorActionPreference = "Stop"
$here = $PSScriptRoot

# Service-account JSON shared with the sales frontend. The account
# (dashboard@keen-scion-499708-j2.iam.gserviceaccount.com) has read access to
# the spreadsheet; keep the file git-ignored.
if (-not $env:SALES_GOOGLE_CREDENTIALS) {
  # Kredensial disalin ke folder ini sendiri (dulu di ..\greenparksales, tapi
  # folder FE itu bisa dipindah/diarsipkan sewaktu-waktu) — no cross-folder dep.
  $env:SALES_GOOGLE_CREDENTIALS = Join-Path $here "keen-scion-499708-j2-b437f6d12fe7.json"
}
if (-not (Test-Path $env:SALES_GOOGLE_CREDENTIALS)) {
  throw "Kredensial Google tidak ditemukan: $env:SALES_GOOGLE_CREDENTIALS"
}

# Live source spreadsheet (DATA PENJUALAN -> detail, META ADS INPUT -> total spent).
if (-not $env:SALES_GSHEET_ID)          { $env:SALES_GSHEET_ID = "1FR0xlB5pEmrbsm3SAtfVAUUG3sDM9MHiseUdTyTD1j8" }
# Auto-sync cadence (minutes). First sync runs immediately on startup.
if (-not $env:SALES_SYNC_INTERVAL_MIN)  { $env:SALES_SYNC_INTERVAL_MIN = "15" }

Write-Host "sales: live-sync ON  sheet=$env:SALES_GSHEET_ID  interval=${env:SALES_SYNC_INTERVAL_MIN}m"
Set-Location $here
go run ./cmd/server
