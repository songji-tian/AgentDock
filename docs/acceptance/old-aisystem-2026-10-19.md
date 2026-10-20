# AgentDock v2 真实验收 — old-Aisystem（2026-10-19）

目标仓库：`/Users/songji/Documents/old-Aisystem/Aisystem`
验收目的：12 命令全链路真机验收，不止跑通核心闭环。

## 验收结果总览（12/12 全部通过）

| 命令 | 结果 | 备注 |
|------|------|------|
| discover | ✅ | 7 服务全对（api/db/feishu-webhook/minio/nginx/redis/web） |
| init | ✅ | 含 EnsureGitignore 修复验证 |
| new | ✅ | 端口分配修复验证（41000→41200） |
| up | ✅ | 6 容器完整起栈，端口 41200 段 |
| down | ✅ | 停栈容器全停 |
| diagnose | ✅ | up/down 两态各 4 项 ok |
| capacity | ✅ | 台账正确 |
| submit | ✅ | --target local，commit 落 dock 分支 |
| gc | ✅ | dry run：no garbage |
| clean | ✅ | "refuses if up" 保护生效 |
| reclaim | ✅ | stop+clean 一次完成，state 清空 |
| merge | ✅ | direct：--no-ff 真并进 main（28a9adc 双 parent） |

## Bug 1：跨项目端口冲突（已修复闭环）

`nextPortBase()` 只遍历本项目 state 的 docks 递增（41000→41200…），从不检查
宿主机全局端口占用。两个不同项目（relations 与 acceptance）各自独立从 41000
起分配 → 端口必撞（up 报 `Bind 41001 failed: already allocated`）。

- 证据：relations state `port_base=41000`；acceptance 也分 41000 段。
- 修复：`internal/ports/ports.go` 新增 `PortFree`/`BlockFree`（net.Listen 探测）；
  `internal/dock/dock.go` 的 `nextPortBase()` 在块内端口被占时 `+stride` 重试。
- 单测：`TestNextPortBaseSkipsOccupiedBlock`。
- 真机：clean 后重新 new，port_base 41000 → 41200，up 完整成功，两栈隔离。

## Bug 2：`.agentdock/` 状态目录从不进 .gitignore（已修复闭环）

AgentDock 的 `init` 只写 config.json，从不往项目 `.gitignore` 追加自己的运行时
状态目录 `.agentdock/`，导致项目 `git status` 长期显示 `?? .agentdock/`，进而
`merge` 的 fail-closed dirty 检查被自己生成的状态目录误判为脏、拒绝合并。

- 证据：flow_test.go 的测试 helper 里**手动** `os.WriteFile(".gitignore", ".agentdock/\n")`
  绕过了这个问题——生产代码从未做，说明是真实缺陷。
- 修复：`internal/config/config.go` 新增 `EnsureGitignore`（幂等追加 `.agentdock/`，
  保留已有条目）；`internal/cli/cli.go` 的 `runInit` 在 `cfg.Save` 后调用。
- 单测：`TestEnsureGitignoreCreatesAndIsIdempotent`、
  `TestEnsureGitignoreKeepsExistingEntries`。
- 真机：重新 init 后 `.gitignore` 第 80 行自动出现 `.agentdock/`，status 里
  `?? .agentdock/` 消失，merge 得以放行。

## 观察点（小瑕疵，非阻塞，建议后续排期）

1. **submit 会 commit override 文件**：`commitWorktree` 用 `git add -A`，把运行时
   生成的 `docker-compose.agentdock.yml`（override，头标 "do not edit"，含端口 remap
   + container_name）一并 commit 进 dock 分支。运行时产物不应进版本库。
2. **auth 检测未写入 config**：`new --use-default-auth` 时提示
   `auth not detected in config; ignoring --admin-* flags`——init/discover 检测到
   admin-login 信号（env:ADMIN_PASSWORD + 7 个 seed 脚本）但未把 `auth.detected=true`
   写回 config.json，导致站级凭据 flag 被静默忽略。

## 环境问题（非 AgentDock bug，已绕过）

- 容器内 DNS 被宿主代理 fake-ip 劫持（deb.debian.org → 198.18.x），清华 pip 源 403，
  `up` 的 backend build 卡死。
- 绕过：复用本地历史镜像 `aisystem-acceptance-{api,web,feishu-webhook}:latest`，
  worktree `docker-compose.yml` 三个服务 `build`→`image`；minio 加
  `profiles: ["disabled"]`（Docker Hub 已删 minio/minio 镜像）。
- 适配是验收环境特化，每次 new 后需重做（已 commit 进 worktree）。

## 环境改动记录（Aisystem 测试仓库）

- main 前进到 `28a9adc`（merge commit，parent 为 b4eaffc + a376f89）。
- `.gitignore` 追加 `.agentdock/`（commit b4eaffc，Bug2 修复的落盘）。
- `data/` 两个运行时报告 md 在 merge 前临时备份到 /tmp 后移回，现已恢复原位。
- 回退点：`git reset --hard b4eaffc` 可撤销本次 merge（保留 .gitignore 修复）。
