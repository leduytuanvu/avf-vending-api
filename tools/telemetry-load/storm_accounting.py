#!/usr/bin/env python3
"""Deterministic storm accounting and Prometheus text parsing for telemetry_storm_load_test.sh.

Not imported by production binaries; used only by the load-test script.
"""
from __future__ import annotations

import argparse
import json
import re
import sys
from typing import Dict, Iterable, List, Tuple


def is_critical_event(idx: int, wave: int, critical_pct: int) -> bool:
    bucket = (idx * 10007 + wave * 7919) % 100
    return bucket < critical_pct


def count_critical_events(machine_count: int, events_per_machine: int, critical_pct: int) -> int:
    n = 0
    for wave in range(1, events_per_machine + 1):
        for idx in range(1, machine_count + 1):
            if is_critical_event(idx, wave, critical_pct):
                n += 1
    return n


def count_duplicate_replays(
    machine_count: int, events_per_machine: int, critical_pct: int, dup_pct: int
) -> int:
    """Extra identical critical publishes (second delivery) planned by the storm script."""
    if dup_pct <= 0:
        return 0
    n = 0
    for wave in range(1, events_per_machine + 1):
        for idx in range(1, machine_count + 1):
            if not is_critical_event(idx, wave, critical_pct):
                continue
            b = (idx * 131 + wave * 171) % 100
            if b < dup_pct:
                n += 1
    return n


# Prometheus text format: metric_name{labels} value OR metric_name value
_LINE_RE = re.compile(
    r"^(?P<name>[a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{(?P<labels>[^}]*)\})?\s+(?P<val>[-+eE0-9.]+|nan|NaN)\s*$"
)


def _parse_labels(label_blob: str) -> Dict[str, str]:
    out: Dict[str, str] = {}
    if not label_blob.strip():
        return out
    # label="value",foo="bar"
    for part in re.findall(r'([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*"((?:[^"\\]|\\.)*)"', label_blob):
        k, v = part
        out[k] = v.replace('\\"', '"')
    return out


def parse_prom_counters(text: str) -> Dict[Tuple[str, Tuple[Tuple[str, str], ...]], float]:
    """Map (metric_name, sorted_label_pairs) -> value (last wins)."""
    out: Dict[Tuple[str, Tuple[Tuple[str, str], ...]], float] = {}
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        m = _LINE_RE.match(line)
        if not m:
            continue
        name = m.group("name")
        labels_raw = m.group("labels") or ""
        val_raw = m.group("val")
        try:
            val = float(val_raw)
        except ValueError:
            continue
        labels = _parse_labels(labels_raw)
        key_l = tuple(sorted(labels.items()))
        out[(name, key_l)] = val
    return out


def counter_delta(
    before: Dict[Tuple[str, Tuple[Tuple[str, str], ...]], float],
    after: Dict[Tuple[str, Tuple[Tuple[str, str], ...]], float],
    metric: str,
    label_filter: Dict[str, str] | None = None,
) -> float:
    """Sum delta across all series matching metric name and optional label subset."""
    label_filter = label_filter or {}

    def matches(labels: Tuple[Tuple[str, str], ...]) -> bool:
        d = dict(labels)
        for k, v in label_filter.items():
            if d.get(k) != v:
                return False
        return True

    keys: Iterable[Tuple[str, Tuple[Tuple[str, str], ...]]] = (
        k for k in before if k[0] == metric and matches(k[1])
    )
    keys = set(keys)
    keys |= {k for k in after if k[0] == metric and matches(k[1])}
    total = 0.0
    for k in keys:
        total += after.get(k, 0.0) - before.get(k, 0.0)
    return total


def max_gauge_value(text: str, metric_prefix: str) -> float:
    m = 0.0
    for line in text.splitlines():
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        mm = _LINE_RE.match(line)
        if not mm or not mm.group("name").startswith(metric_prefix):
            continue
        try:
            v = float(mm.group("val"))
        except ValueError:
            continue
        if v > m:
            m = v
    return m


def cmd_expect(args: argparse.Namespace) -> None:
    crit_pct = int(args.critical_pct)
    mc = int(args.machine_count)
    epm = int(args.events_per_machine)
    dup_pct = int(args.duplicate_pct)
    exp = count_critical_events(mc, epm, crit_pct)
    dup_n = count_duplicate_replays(mc, epm, crit_pct, dup_pct)
    out = {
        "critical_expected_unique": exp,
        "critical_duplicate_replays_planned": dup_n,
        "critical_publishes_planned": exp + dup_n,
    }
    print(json.dumps(out))


def cmd_delta(args: argparse.Namespace) -> None:
    with open(args.before, encoding="utf-8") as f:
        btxt = f.read()
    with open(args.after, encoding="utf-8") as f:
        atxt = f.read()
    before = parse_prom_counters(btxt)
    after = parse_prom_counters(atxt)

    def d(metric: str, lf: Dict[str, str] | None = None) -> float:
        return counter_delta(before, after, metric, lf)

    out = {
        "ingest_received_critical_no_drop_delta": d(
            "avf_telemetry_ingest_received_total", {"channel": "critical_no_drop"}
        ),
        "ingest_received_telemetry_delta": d("avf_telemetry_ingest_received_total", {"channel": "telemetry"}),
        "mqtt_dispatch_telemetry_ok_delta": d(
            "avf_mqtt_ingest_dispatch_total", {"kind": "telemetry", "result": "ok"}
        ),
        "mqtt_dispatch_telemetry_err_delta": d(
            "avf_mqtt_ingest_dispatch_total", {"kind": "telemetry", "result": "error"}
        ),
        "ingest_publish_failures_delta": d("avf_telemetry_ingest_publish_failures_total"),
        "ingest_critical_missing_identity_delta": d("avf_telemetry_ingest_critical_missing_identity_total"),
        "telemetry_idempotency_conflict_delta": d("avf_telemetry_idempotency_conflict_total"),
        "telemetry_duplicate_edge_delta": d("avf_telemetry_duplicate_total", {"reason": "edge_event"}),
        "telemetry_duplicate_inventory_delta": d(
            "avf_telemetry_duplicate_total", {"reason": "inventory_event"}
        ),
        "telemetry_duplicate_idem_replay_delta": d(
            "avf_telemetry_duplicate_total", {"reason": "idempotency_replay"}
        ),
        "telemetry_duplicate_critical_rollup_delta": d(
            "avf_telemetry_duplicate_total", {"reason": "critical_metrics_rollup"}
        ),
        "projection_failures_handler_delta": d(
            "avf_telemetry_projection_failures_total", {"reason": "handler_err"}
        ),
        "max_consumer_lag_after": max_gauge_value(atxt, "avf_telemetry_consumer_lag"),
    }
    print(json.dumps(out))


def main(argv: List[str] | None = None) -> int:
    p = argparse.ArgumentParser()
    sub = p.add_subparsers(dest="cmd", required=True)

    pe = sub.add_parser("expect")
    pe.add_argument("--machine-count", type=int, required=True)
    pe.add_argument("--events-per-machine", type=int, required=True)
    pe.add_argument("--critical-pct", type=int, required=True, help="0-99 bucket threshold")
    pe.add_argument("--duplicate-pct", type=int, default=0, help="0-99 for duplicate replay subset")
    pe.set_defaults(func=cmd_expect)

    pd = sub.add_parser("delta")
    pd.add_argument("--before", required=True)
    pd.add_argument("--after", required=True)
    pd.set_defaults(func=cmd_delta)

    args = p.parse_args(argv)
    args.func(args)
    return 0


if __name__ == "__main__":
    sys.exit(main())
