#!/usr/bin/env python3
"""Reconciliation script: compare Go escrow state with Python wallet state.

Usage:
    python scripts/reconcile_escrow.py [--go-url http://go-escrow:8081]
"""
import argparse
import asyncio
import os
import sys
from decimal import Decimal
from uuid import UUID

import httpx
import redis.asyncio as aioredis
from sqlalchemy import select, text
from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession

GO_URL = os.getenv("GO_ESCROW_BASE_URL", "http://localhost:8081")
REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379/0")
DB_URL = os.getenv("DB_URL", "sqlite+aiosqlite:///./marketplace.db")
ESCROW_USER_ID = UUID("1133d650-e7d4-41de-8877-c359682903a4")


async def reconcile():
    parser = argparse.ArgumentParser(description="Reconcile Go escrow with Python wallet")
    parser.add_argument("--go-url", default=GO_URL)
    parser.add_argument("--redis-url", default=REDIS_URL)
    parser.add_argument("--db-url", default=DB_URL)
    args = parser.parse_args()

    engine = create_async_engine(args.db_url)
    r = aioredis.from_url(args.redis_url, decode_responses=True)

    async with engine.begin() as conn:
        rows = await conn.execute(
            text("SELECT id, amount, status, buyer_id, seller_id FROM orders WHERE status IN ('funded', 'in_progress')")
        )
        orders = rows.fetchall()

    if not orders:
        print("No active escrow orders found (status=funded or in_progress). Nothing to reconcile.")
        return

    print(f"Found {len(orders)} active orders. Checking...\n")

    mismatches = []
    async with AsyncSession(engine) as session:

        for row in orders:
            order_id = str(row[0])
            amount = Decimal(str(row[1]))
            status = row[2]

            escrow_id = await r.get(f"escrow:order:{order_id}:id")
            if not escrow_id:
                print(f"  ? {order_id}: no escrow_id in Redis cache (order was paid before this deploy?)")
                mismatches.append((order_id, "no_escrow_id", status))
                continue

            try:
                async with httpx.AsyncClient(timeout=5) as client:
                    resp = await client.get(f"{args.go_url}/v1/escrow/{escrow_id}")
                    if resp.status_code == 404:
                        print(f"  ! {order_id}: Go escrow {escrow_id} NOT FOUND")
                        mismatches.append((order_id, "go_not_found", status))
                        continue
                    resp.raise_for_status()
                    go_data = resp.json().get("data", {})
            except Exception as e:
                print(f"  ! {order_id}: Go request failed: {e}")
                mismatches.append((order_id, f"go_error: {e}", status))
                continue

            go_status = go_data.get("status")
            go_balance = Decimal(str(go_data.get("balance", "0")))

            if go_status == "CANCELLED":
                print(f"  ~ {order_id}: order={status}, Go=CANCELLED (stale mapping, order was cancelled in Python but cache still points to this escrow)")
                mismatches.append((order_id, f"go_cancelled_order_{status}", status))
                continue

            if go_status == "RELEASED":
                print(f"  ~ {order_id}: order={status}, Go=RELEASED (stale mapping)")
                mismatches.append((order_id, f"go_released_order_{status}", status))
                continue

            expected_go_status = status.upper().replace("-", "_") if status != "funded" else "FUNDED"
            if go_status != expected_go_status:
                print(f"  MISMATCH {order_id}: order.status={status}, go.status={go_status}")
                mismatches.append((order_id, f"status_mismatch: order={status} go={go_status}", status))

            wallet_row = await conn.execute(
                text("SELECT balance FROM wallets WHERE user_id = :uid"),
                {"uid": str(ESCROW_USER_ID)},
            )
            escrow_balance = Decimal(str(wallet_row.scalar() or "0"))

            if escrow_balance < amount:
                print(f"  MISMATCH {order_id}: escrow_wallet={escrow_balance} < order.amount={amount} (possible rollback or incomplete fund)")
                mismatches.append((order_id, f"insufficient_escrow_balance: wallet={escrow_balance} needed={amount}", status))

    await r.close()
    await engine.dispose()

    print("\n--- Summary ---")
    if mismatches:
        print(f"Found {len(mismatches)} issue(s):")
        for oid, issue, st in mismatches:
            print(f"  {oid}: [{st}] {issue}")
        sys.exit(1)
    else:
        print("All escrow states are consistent. No issues found.")
        sys.exit(0)


if __name__ == "__main__":
    asyncio.run(reconcile())
