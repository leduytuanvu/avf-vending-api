> **Relocated from avf-vending-system** — canonical home: `avf-vending-api/docs/production/`. System copy removed 2026-06-16.

﻿# FINAL Production Cash-Only Market Readiness

- **finalVerdict:** `NOT_READY_FOR_MARKET_CASH_SALES`
- **marketReady:** False
- **commandCenterFullReal:** `FAIL` — Bill full-real **8/31** on `PFT9UY4Y59` (2026-06-08): harness parser patch verified for rows 1–7; stopped at `FAIL_BILL_COMMAND_tx_present_rx_timeout:bill-credit-map` (BILL TX sent, no RX within timeout — hardware/protocol, not parser tap bug)
- **serialDataIntegrityGate (G2):** `IMPLEMENTED_NOT_VERIFIED` — soak script present; no fresh `SERIAL_DATA_INTEGRITY_RESULT.md` this session
- **MR-G13 result_category:** `SKIPPED_PREREQUISITE` (Bill+TCN UI matrix incomplete; G2 not verified)
- **combinedTier if G2-only pass:** `SERIAL_OK_COMMAND_CENTER_FAIL` (G2 cannot unlock MR-G13 alone)

## P0 PDF-exact repair (this session)

| Layer | Status |
|-------|--------|
| Part A truth reports | **DONE** — `BILL_ICT_BC_PDF_COMMAND_TRUTH.md`, `TCN_PDF_COMMAND_TRUTH.md`, `OLD_APP_KNOWN_GOOD_COMMAND_FIXTURES.md`, `PDF_VS_NEW_APP_COMMAND_GAPS.md` |
| Bill 0x1C/0x1E 1-byte count | **DONE** — `dispenseBillCount` / `transferBillCount` |
| Bill 0x15 2-byte Y1/Y2 mask | **DONE** — `IctBillTypeEnableMask` + golden tests |
| Bill raw / monitor chains | **DONE** — `sendRawFrame`, dispense/transfer + 0x1D/0x20 monitor loops |
| TCN belt=0x68 / spring=0x74 | **DONE** — Kotlin + manifest + PS `CommandOpcodeMap` + `CommandTapHints` |
| tcn-all-params-group / tcn-raw | **DONE** — executor branches + `sendRawFrame` |
| Full matrix schema | **DONE** — `expectedTxHex`, `actualTxHex`, `txMatchesPdf`, `sourcePath` in `New-CommandMatrixRow` |
| Unit tests (hardware-bill/tcn) | **PASS** — golden + mask tests green |
| Production APK | **INSTALLED** on `PFT9UY4Y59` (`installTcnProductionDebug`) |
| Live full matrix (31 Bill + 32 TCN UI) | **PARTIAL** — Bill **8/31** (`TECHNICIAN_BILL_FULL_REAL_COMMAND_RESULT.md`); parser-only rows not yet reached; `bill-credit-map` RX timeout |
| Bill parser harness (P0) | **DONE** — `PROTOCOL_PARSER` auto-rows, ASCII search, bookmark tap proof, `bill-raw` → `EXPERT_UTILITY`, commandId integrity gate |

## Prerequisite chain for MR-G13

Bill UI matrix complete → TCN UI matrix complete → G2 soak PASS → MR-G13.

**Do not** mark `FULL_REAL_COMMAND_VALIDATION_PASS` until `FULL_COMMAND_RESPONSE_MATRIX.md` shows every executable Bill/TCN row as `UI_COMMAND` with TX/RX evidence and zero software `FAIL_*`.

## Gate summary

| Gate | Verdict | result_category |
| --- | --- | --- |
| MR-G0 | PASS | PASS |
| MR-G1 | PASS | PASS |
| MR-G2 | PASS | PASS (legacy market gate) |
| **G2 Serial Integrity** | NOT VERIFIED | `IMPLEMENTED_NOT_VERIFIED` |
| **MR-G13 Command synthesis** | SKIPPED | `SKIPPED_PREREQUISITE` |
| MR-G8 | BLOCKED | `BLOCKED_PHYSICAL_PRECONDITION` |
| MR-G9 | BLOCKED | `BLOCKED_PHYSICAL_PRECONDITION` |
| MR-G10–12 | SKIPPED/BLOCKED | prerequisites |

## P0 Bill parser harness rerun (2026-06-08, PFT9UY4Y59)

| Metric | Result |
|--------|--------|
| Entry | **PASS** (`coordinate_bounds`) |
| Rows tested | **8 / 31** |
| Live PASS (TX/RX) | **6** — `bill-manufacturer` … `bill-decimal` (PDF-matching TX) |
| Timeout w/ evidence | **1** — `bill-country` (`LIVE_EXECUTION_TIMEOUT_WITH_EVIDENCE`) |
| Software FAIL | **1** — `bill-credit-map` (`tx_present_rx_timeout`) |
| Protocol parser auto-rows | **Not reached** (rows 12–14) — prior `bill-proto-reply` tap bug **not reproduced** |
| Report | `avf-vending-app/reports/TECHNICIAN_BILL_FULL_REAL_COMMAND_RESULT.md` |
| Artifacts | `avf-vending-app/build/technician-bill-full-command-center-20260608-022604/` |

## Honest blockers

- Bill full-real matrix incomplete: **`bill-credit-map` BILL RX timeout** (investigate acceptor / 0x14 response)
- Protocol parser rows (3), remaining PDF executable (19), expert utility (`bill-raw`) not exercised this run
- Full Bill/TCN UI command-center matrix not completed on device
- G2 live soak not re-run this session
- Real BILL cash + TCN dispense evidence still required for market readiness

**NOT READY_FOR_MARKET_CASH_SALES.**
