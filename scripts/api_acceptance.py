#!/usr/bin/env python3
"""Black-box acceptance test for the CDK shop API.

The service must run with APP_ENV=development and PAYMENT_PROVIDER=mock.
This script creates isolated records with a timestamp suffix and does not
delete data. Pass --database to also verify the SQLite persistence state.
"""

from __future__ import annotations

import argparse
import json
import os
import sqlite3
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any


class AcceptanceError(RuntimeError):
    pass


class API:
    def __init__(self, base_url: str) -> None:
        self.base_url = base_url.rstrip("/")

    def request(
        self,
        method: str,
        path: str,
        *,
        body: dict[str, Any] | None = None,
        token: str | None = None,
        expected: int | tuple[int, ...] = 200,
    ) -> Any:
        expected_codes = (expected,) if isinstance(expected, int) else expected
        data = None if body is None else json.dumps(body).encode("utf-8")
        headers = {"Accept": "application/json"}
        if data is not None:
            headers["Content-Type"] = "application/json"
        if token:
            headers["Authorization"] = f"Bearer {token}"
        request = urllib.request.Request(
            f"{self.base_url}{path}", data=data, headers=headers, method=method
        )
        try:
            response = urllib.request.urlopen(request, timeout=15)
            status = response.status
            raw = response.read()
        except urllib.error.HTTPError as exc:
            status = exc.code
            raw = exc.read()
        except urllib.error.URLError as exc:
            raise AcceptanceError(f"cannot reach {self.base_url}: {exc.reason}") from exc
        if raw:
            decoded = raw.decode("utf-8", errors="replace")
            try:
                parsed = json.loads(decoded)
            except json.JSONDecodeError:
                parsed = decoded
        else:
            parsed = None
        if status not in expected_codes:
            raise AcceptanceError(
                f"{method} {path}: expected {expected_codes}, got {status}: {parsed}"
            )
        return parsed


def check(value: bool, message: str) -> None:
    if not value:
        raise AcceptanceError(message)


def passed(name: str) -> None:
    print(f"PASS  {name}")


def order_no(result: dict[str, Any]) -> str:
    return str(result["order"]["order_no"])


def verify_database(
    database: Path,
    *,
    user_id: int,
    order_numbers: list[str],
    card_secrets: list[str],
    guest_contact: str,
    query_password: str,
) -> None:
    check(database.is_file(), f"database does not exist: {database}")
    connection = sqlite3.connect(f"file:{database.as_posix()}?mode=ro", uri=True)
    try:
        balance = connection.execute(
            "SELECT balance_cents FROM users WHERE id = ?", (user_id,)
        ).fetchone()
        check(balance == (4000,), f"unexpected persisted balance: {balance}")

        placeholders = ",".join("?" for _ in order_numbers)
        rows = connection.execute(
            f"SELECT payment_method, status, user_id FROM orders "
            f"WHERE order_no IN ({placeholders}) ORDER BY id",
            order_numbers,
        ).fetchall()
        check(len(rows) == 3, f"expected three persisted orders, got {len(rows)}")
        check(all(row[1] == "completed" for row in rows), "not all orders completed")
        check(sum(row[0] == "balance" for row in rows) == 1, "balance order mismatch")
        check(sum(row[0] == "online" for row in rows) == 2, "online order mismatch")
        check(sum(row[2] is None for row in rows) == 1, "guest order ownership mismatch")

        sold = connection.execute(
            "SELECT COUNT(*) FROM cards WHERE status = 'sold' AND sold_order_id IS NOT NULL"
        ).fetchone()[0]
        check(sold >= 3, f"expected at least three sold cards, got {sold}")
        for secret in card_secrets:
            count = connection.execute(
                "SELECT COUNT(*) FROM cards WHERE secret_ciphertext = ?", (secret,)
            ).fetchone()[0]
            check(count == 0, "card secret was persisted as plaintext")

        guest = connection.execute(
            "SELECT contact_ciphertext, query_password_hash FROM orders "
            "WHERE order_no = ?",
            (order_numbers[-1],),
        ).fetchone()
        check(guest is not None, "guest order missing from database")
        check(guest[0] != guest_contact, "guest contact was persisted as plaintext")
        check(guest[1] != query_password, "guest query password was persisted as plaintext")

        payments = connection.execute(
            f"SELECT COUNT(*) FROM payments p JOIN orders o ON o.id = p.order_id "
            f"WHERE o.order_no IN ({placeholders}) AND p.status = 'success'",
            order_numbers,
        ).fetchone()[0]
        check(payments == 2, f"expected two successful payments, got {payments}")

        forbidden_tables = connection.execute(
            "SELECT name FROM sqlite_master WHERE type = 'table' "
            "AND (name LIKE '%withdraw%' OR name LIKE '%refund%')"
        ).fetchall()
        check(not forbidden_tables, f"unexpected refund/withdraw tables: {forbidden_tables}")
    finally:
        connection.close()
    passed("SQLite balances, orders, payments, and encrypted fields")


def run(args: argparse.Namespace) -> None:
    api = API(args.base_url)
    suffix = str(time.time_ns())[-12:]

    health = api.request("GET", "/healthz")
    check(health == {"status": "ok"}, f"unexpected health response: {health}")
    passed("health check")

    login = api.request(
        "POST",
        "/api/v1/auth/login",
        body={"login": args.admin_email, "password": args.admin_password},
    )
    admin_token = login["token"]
    passed("administrator login")

    platform = api.request(
        "POST",
        "/api/v1/admin/platforms",
        token=admin_token,
        expected=201,
        body={"code": f"qa-{suffix}", "name": f"QA Platform {suffix}", "status": "active"},
    )
    product = api.request(
        "POST",
        "/api/v1/admin/products",
        token=admin_token,
        expected=201,
        body={
            "platform_id": platform["id"],
            "name": f"QA Product {suffix}",
            "slug": f"qa-product-{suffix}",
            "status": "active",
        },
    )
    sku = api.request(
        "POST",
        "/api/v1/admin/skus",
        token=admin_token,
        expected=201,
        body={
            "product_id": product["id"],
            "name": "QA SKU",
            "sale_price_cents": 1000,
            "status": "active",
        },
    )
    card_secrets = [
        f"QA-{suffix}-BALANCE",
        f"QA-{suffix}-MEMBER-ONLINE",
        f"QA-{suffix}-GUEST-ONLINE",
    ]
    imported = api.request(
        "POST",
        "/api/v1/admin/card-batches",
        token=admin_token,
        expected=201,
        body={
            "sku_id": sku["id"],
            "filename": "api-acceptance.txt",
            "cards": card_secrets + [card_secrets[0], ""],
        },
    )
    batch = imported["batch"]
    check(batch["success_count"] == 3, f"unexpected import result: {batch}")
    check(batch["duplicate_count"] == 1, f"duplicate card was not rejected: {batch}")
    check(batch["invalid_count"] == 1, f"invalid card was not rejected: {batch}")
    passed("catalog creation and encrypted card import")

    user_password = f"Qa!{suffix}x"
    registered = api.request(
        "POST",
        "/api/v1/auth/register",
        expected=201,
        body={
            "username": f"qa_{suffix}",
            "email": f"qa_{suffix}@example.test",
            "password": user_password,
        },
    )
    user_token = registered["token"]
    user_id = int(registered["user"]["id"])
    api.request(
        "POST",
        f"/api/v1/admin/users/{user_id}/balance-adjustments",
        token=admin_token,
        body={
            "direction": "in",
            "amount_cents": 5000,
            "reason": "API acceptance test",
            "idempotency_key": f"credit-{suffix}",
        },
    )
    passed("member registration and administrator balance credit")

    balance_body = {
        "sku_id": sku["id"],
        "quantity": 1,
        "payment_method": "balance",
        "idempotency_key": f"balance-{suffix}",
    }
    balance_order = api.request(
        "POST", "/api/v1/me/orders", token=user_token, expected=201, body=balance_body
    )
    check(balance_order["order"]["status"] == "completed", "balance order not completed")
    check(balance_order["cards"] == [card_secrets[0]], "balance order card mismatch")
    repeated = api.request(
        "POST", "/api/v1/me/orders", token=user_token, expected=200, body=balance_body
    )
    check(order_no(repeated) == order_no(balance_order), "balance idempotency failed")
    profile = api.request("GET", "/api/v1/me", token=user_token)
    check(profile["balance_cents"] == 4000, "balance was deducted more than once")
    passed("registered member balance payment and idempotency")

    online_order = api.request(
        "POST",
        "/api/v1/me/orders",
        token=user_token,
        expected=201,
        body={
            "sku_id": sku["id"],
            "quantity": 1,
            "payment_method": "online",
            "idempotency_key": f"member-online-{suffix}",
        },
    )
    check(online_order["order"]["status"] == "pending_payment", "online order not pending")
    check(not online_order.get("cards"), "unpaid member order exposed a card")
    paid_member = api.request(
        "POST", f"/api/v1/dev/payments/{order_no(online_order)}/pay"
    )
    check(paid_member["order"]["status"] == "completed", "member payment not completed")
    check(paid_member["cards"] == [card_secrets[1]], "member online card mismatch")
    passed("registered member online payment")

    api.request(
        "POST",
        "/api/v1/guest/orders",
        expected=400,
        body={
            "sku_id": sku["id"],
            "quantity": 1,
            "idempotency_key": f"guest-invalid-{suffix}",
            "query_password": "query123",
        },
    )
    api.request(
        "POST",
        "/api/v1/me/orders",
        expected=401,
        body={
            "sku_id": sku["id"],
            "quantity": 1,
            "payment_method": "balance",
            "idempotency_key": f"guest-balance-{suffix}",
        },
    )
    guest_contact = f"9{suffix[:9]}"
    query_password = f"Qp{suffix[:8]}"
    guest_order = api.request(
        "POST",
        "/api/v1/guest/orders",
        expected=201,
        body={
            "sku_id": sku["id"],
            "quantity": 1,
            "payment_method": "balance",
            "idempotency_key": f"guest-online-{suffix}",
            "contact_type": "qq",
            "contact": guest_contact,
            "query_password": query_password,
        },
    )
    check(guest_order["order"]["payment_method"] == "online", "guest was not forced online")
    check(not guest_order.get("cards"), "unpaid guest order exposed a card")
    query_body = {
        "order_no": order_no(guest_order),
        "contact_type": "qq",
        "contact": guest_contact,
        "query_password": query_password,
    }
    pending_guest = api.request("POST", "/api/v1/guest/orders/query", body=query_body)
    check(not pending_guest.get("cards"), "guest query exposed card before payment")
    wrong_query = dict(query_body)
    wrong_query["query_password"] = "wrong-password"
    api.request(
        "POST", "/api/v1/guest/orders/query", body=wrong_query, expected=403
    )
    api.request("POST", f"/api/v1/dev/payments/{order_no(guest_order)}/pay")
    completed_guest = api.request("POST", "/api/v1/guest/orders/query", body=query_body)
    check(completed_guest["order"]["status"] == "completed", "guest order not completed")
    check(completed_guest["cards"] == [card_secrets[2]], "guest card mismatch")
    passed("guest online-only payment and protected order query")

    api.request("POST", "/api/v1/me/withdrawals", token=user_token, expected=404)
    api.request("POST", "/api/v1/me/refunds", token=user_token, expected=404)
    passed("withdrawal and refund APIs are absent")

    if args.database:
        verify_database(
            Path(args.database).resolve(),
            user_id=user_id,
            order_numbers=[
                order_no(balance_order),
                order_no(online_order),
                order_no(guest_order),
            ],
            card_secrets=card_secrets,
            guest_contact=guest_contact,
            query_password=query_password,
        )

    print("\nAll API acceptance checks passed.")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--base-url", default=os.getenv("CARD_BASE_URL", "http://127.0.0.1:3000")
    )
    parser.add_argument(
        "--admin-email", default=os.getenv("BOOTSTRAP_ADMIN_EMAIL", "admin@example.com")
    )
    parser.add_argument(
        "--admin-password",
        default=os.getenv("BOOTSTRAP_ADMIN_PASSWORD", "AdminPass123!"),
    )
    parser.add_argument("--database", help="optional SQLite database path")
    return parser.parse_args()


if __name__ == "__main__":
    try:
        run(parse_args())
    except (AcceptanceError, KeyError, TypeError, ValueError) as exc:
        print(f"FAIL  {exc}", file=sys.stderr)
        raise SystemExit(1)
