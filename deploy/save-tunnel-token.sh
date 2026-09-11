#!/usr/bin/env bash
set -euo pipefail

TF_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
install -d -m 0700 "$TF_ROOT/.secrets"
IFS= read -rsp '粘贴 Cloudflare Docker 安装命令或 Token，然后按回车: ' TF_INPUT
printf '\n'
printf '%s' "$TF_INPUT" | python3 -c '
import base64, json, os, shlex, sys, tempfile
from pathlib import Path

try:
    parts = shlex.split(sys.stdin.read().strip())
    if "--token" in parts:
        token = parts[parts.index("--token") + 1]
    elif len(parts) == 1:
        token = parts[0]
    else:
        raise ValueError()
    data = json.loads(base64.b64decode(token, validate=True))
    if not all(isinstance(data.get(k), str) and data[k] for k in ("a", "t", "s")):
        raise ValueError()
except (ValueError, IndexError, TypeError):
    sys.exit("输入不是有效的 Tunnel Token；请复制 Docker 安装命令后重新运行。")

target = Path(sys.argv[1])
fd, temporary = tempfile.mkstemp(prefix=".token-", dir=target.parent)
try:
    with os.fdopen(fd, "w") as f:
        f.write(token)
        f.flush()
        os.fsync(f.fileno())
    os.chmod(temporary, 0o444)
    os.replace(temporary, target)
finally:
    if os.path.exists(temporary):
        os.unlink(temporary)
print("Token 已保存，未输出凭证内容。")
' "$TF_ROOT/.secrets/cloudflared-token"
unset TF_INPUT
