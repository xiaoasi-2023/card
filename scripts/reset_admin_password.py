#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""重置 card 管理端密码（本地 / 服务器通用）。

适用场景：
- 忘记管理员密码
- 线上库由本地 card.db 拷贝，密码与本地一致但仍想统一改掉

说明：
- 只改 users 表里 role=admin 的 password_hash
- 不改动订单、卡密、商品等业务数据
- BOOTSTRAP_ADMIN_PASSWORD 环境变量对已有 admin 无效，必须改库

依赖：
  pip install bcrypt
  （可选）系统自带 sqlite3 命令行不需要

示例：
  # 查看当前管理员
  python3 scripts/reset_admin_password.py --db /app/data/card.db --list

  # 重置（交互输入新密码，避免出现在 shell 历史）
  python3 scripts/reset_admin_password.py --db /app/data/card.db

  # 非交互（注意历史记录风险）
  python3 scripts/reset_admin_password.py \\
    --db /app/data/card.db \\
    --email admin@example.com \\
    --password '你的新密码' \\
    --yes
"""

from __future__ import annotations

import argparse
import getpass
import sqlite3
import sys
from datetime import datetime, timezone
from pathlib import Path


def _require_bcrypt():
    try:
        import bcrypt  # type: ignore
    except ImportError as exc:
        raise SystemExit(
            "缺少 bcrypt。请先安装：pip install bcrypt  或  pip3 install bcrypt"
        ) from exc
    return bcrypt


def connect(db_path: Path) -> sqlite3.Connection:
    if not db_path.exists():
        raise SystemExit(f"数据库不存在: {db_path}")
    con = sqlite3.connect(str(db_path))
    con.row_factory = sqlite3.Row
    return con


def list_admins(con: sqlite3.Connection) -> list[sqlite3.Row]:
    return list(
        con.execute(
            """
            SELECT id, username, email, role, status, created_at, updated_at
            FROM users
            WHERE role = 'admin'
            ORDER BY id
            """
        )
    )


def find_admin(con: sqlite3.Connection, email: str | None, user_id: int | None) -> sqlite3.Row:
    if user_id is not None:
        row = con.execute(
            "SELECT id, username, email, role, status FROM users WHERE id = ? AND role = 'admin'",
            (user_id,),
        ).fetchone()
        if row is None:
            raise SystemExit(f"找不到 id={user_id} 的 admin 用户")
        return row

    if email:
        row = con.execute(
            """
            SELECT id, username, email, role, status
            FROM users
            WHERE role = 'admin' AND lower(email) = lower(?)
            """,
            (email.strip(),),
        ).fetchone()
        if row is None:
            raise SystemExit(f"找不到 email={email!r} 的 admin 用户")
        return row

    rows = list_admins(con)
    if not rows:
        raise SystemExit("库中没有任何 role=admin 的用户")
    if len(rows) > 1:
        print("存在多个 admin，请用 --email 或 --id 指定：")
        for row in rows:
            print(f"  id={row['id']}  username={row['username']}  email={row['email']}")
        raise SystemExit(2)
    return rows[0]


def hash_password(plain: str) -> str:
    bcrypt = _require_bcrypt()
    # 与 Go golang.org/x/crypto/bcrypt.DefaultCost(=10) 对齐
    hashed = bcrypt.hashpw(plain.encode("utf-8"), bcrypt.gensalt(rounds=10))
    return hashed.decode("utf-8")


def reset_password(con: sqlite3.Connection, admin_id: int, password_hash: str) -> None:
    now = datetime.now(timezone.utc).isoformat()
    cur = con.execute(
        """
        UPDATE users
        SET password_hash = ?, updated_at = ?
        WHERE id = ? AND role = 'admin'
        """,
        (password_hash, now, admin_id),
    )
    if cur.rowcount != 1:
        raise SystemExit("更新失败：未影响任何 admin 行（请确认库路径与用户）")
    con.commit()


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="重置 card 管理端密码（SQLite users.password_hash）"
    )
    parser.add_argument(
        "--db",
        required=True,
        help="card.db 路径。Docker 常见: /app/data/card.db 或宿主机挂载目录下的 card.db",
    )
    parser.add_argument("--list", action="store_true", help="只列出 admin，不改密码")
    parser.add_argument("--email", default="", help="目标管理员邮箱，默认 admin@example.com")
    parser.add_argument("--id", type=int, default=None, help="目标管理员用户 id")
    parser.add_argument(
        "--password",
        default="",
        help="新密码；不传则交互输入（推荐，避免进 shell 历史）",
    )
    parser.add_argument("--yes", action="store_true", help="跳过确认提示")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    db_path = Path(args.db).expanduser().resolve()
    con = connect(db_path)

    print(f"数据库: {db_path}")
    admins = list_admins(con)
    if not admins:
        raise SystemExit("库中没有 admin 用户。可配置 BOOTSTRAP_ADMIN_* 后空库启动一次。")

    print("当前 admin：")
    for row in admins:
        print(
            f"  id={row['id']}  username={row['username']!r}  email={row['email']!r}  "
            f"status={row['status']}  updated_at={row['updated_at']}"
        )

    if args.list:
        return 0

    email = (args.email or "").strip() or None
    if email is None and args.id is None and len(admins) == 1:
        # 单 admin 时默认选中
        target = admins[0]
    else:
        if email is None and args.id is None:
            email = "admin@example.com"
        target = find_admin(con, email=email, user_id=args.id)

    plain = args.password
    if not plain:
        plain = getpass.getpass("新密码: ")
        confirm = getpass.getpass("再输入一次: ")
        if plain != confirm:
            raise SystemExit("两次密码不一致")
    if len(plain) < 8:
        raise SystemExit("密码至少 8 位")

    print(
        f"\n将重置: id={target['id']}  username={target['username']!r}  email={target['email']!r}"
    )
    if not args.yes:
        answer = input("确认写入数据库？输入 yes 继续: ").strip().lower()
        if answer != "yes":
            print("已取消")
            return 1

    new_hash = hash_password(plain)
    reset_password(con, int(target["id"]), new_hash)

    # 自检
    bcrypt = _require_bcrypt()
    row = con.execute(
        "SELECT password_hash FROM users WHERE id = ?", (target["id"],)
    ).fetchone()
    if not bcrypt.checkpw(plain.encode("utf-8"), row["password_hash"].encode("utf-8")):
        raise SystemExit("写入后自检失败，请勿使用该密码登录，检查后重试")

    print("重置成功。")
    print(f"登录账号: {target['email']}  或  {target['username']}")
    print("请立即用新密码登录管理端；登录成功后建议再改一次密码。")
    print("注意：改 BOOTSTRAP_ADMIN_PASSWORD 环境变量不会覆盖已有 admin。")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except KeyboardInterrupt:
        print("\n已中断", file=sys.stderr)
        raise SystemExit(130)
