# AgentDock

**为 AI Coding Agent 提供独立、可复现的并行开发环境，且由 Agent 驱动使用的一套系统。**

AgentDock 让多个 AI 编码助手能在同一个仓库上并行工作：每个任务拥有自己的
git worktree、自己的端口段、自己的 Docker 栈。互不干扰、可复现、可回收。

> **AgentDock 是一个 agent，不是让你背命令的 CLI。** 你不该自己敲 `agentdock
> ...` 命令——那样等于拿错了用法。正确用法是把它**注册进你的 coding agent**，
> 由 agent 读它的说明书、做判断，再驱动引擎执行。如果你现在正打算自己敲
> `agentdock ...`，先停下来，往下看"快速开始"。

## 它能干什么

- 同一个仓库上多个任务**并行**工作，各自隔离。
- 每个 dock 有独立的**端口段**，默认排除 80/443（主栈专用）。
- 每次开栈**可复现**：同样的配置得到同样的栈。
- 任务结束**可回收**：一个命令拆掉 worktree 和容器。

## 工作原理（两个半件）

```
你的 coding agent（判断）
      │ 读 AGENT.md + skills/
      ▼
agentdock 引擎（执行 + 校验，确定性二进制）
```

- **agent 半件**（`agent/`）：身份 + 铁律 + 流程。告诉 agent 怎么想。
- **engine 半件**（`cmd/` + `internal/`）：确定性二进制。负责读事实、执行、校验、拒绝。

分工铁律：**agent 提方案，引擎验事实。** 你（agent）漏了一个端口或变量，
引擎必须拦住并拒绝——这正是这个拆分的意义。

## 快速开始（唯一主路径：注册到你的 agent）

```bash
# 1. 安装引擎二进制
go install github.com/songji-tian/agentdock/cmd/agentdock@latest

# 2. 克隆 agent 半件到稳定位置
git clone git@github.com:songji-tian/AgentDock.git ~/.agentdock/

# 3. 在你的项目根放一行指针，让你的 coding agent 去读它
#   Codex → AGENTS.md；Claude Code → CLAUDE.md；Cursor → .cursor/rules/
#   内容（一行）：
#   Before any environment/worktree/port/cleanup work, read
#   ~/.agentdock/agent/AGENT.md and follow its skills.

# 4. 验证：问你的 agent "the one rule?"，它应回答
#   "Never guess. Verify, or refuse."
```

注册完成后，你在项目里说一句"set up AgentDock in this repo"（在这个仓库里
装 AgentDock），agent 会跑 bootstrap skill：发现你的栈 → 和你核对一次 →
写入配置 → 验证。这就是全部接入流程。

> 如果你的运行时没有"skills"概念，`SKILL.md` 仍是可读的说明书——把相关
> 文件贴进你的提示词即可。这套契约不依赖任何厂商特性。

## 命令（供 agent 调用的参考，不是给人背的）

| 命令 | 作用 |
|---|---|
| `agentdock discover` | 确定性解析 compose，输出服务/端口/容器名/必需变量 |
| `agentdock init --from <清单>` | 写入空配置（服务名单全靠 `--from`，永不写死） |
| `agentdock new` | 开一个 dock（worktree + 端口段） |
| `agentdock up` | 起栈（执行前校验，占位配置会被拒绝） |
| `agentdock down` | 停栈 |
| `agentdock clean` | 拆 dock |
| `agentdock capacity` | 查看端口台账和剩余 dock 槽位 |
| `agentdock diagnose` | 检查单个 dock 的一致性（worktree / override / .env / 台账） |
| `agentdock gc [--yes]` | 找出（--yes 则回收）垃圾 dock |
| `agentdock reclaim` | 一条命令停栈 + 拆 |
| `agentdock submit` | 提交 dock 的成果；`--target github` 则推送 |
| `agentdock merge` | 把 dock 分支合回主线（直接合并或开 PR） |
| `agentdock version` | 打印版本 |

## 项目结构

```
agent/           agent 半件（AGENT.md、REGISTRATION.md、skills/、knowledge/）
cmd/agentdock/   引擎入口
internal/        引擎实现（cli、config、discover、ports、lifecycle、state）
docs/            规格（SPEC.md）与架构决策记录（adr/）
```

## 状态

核心闭环（discover / init / new / up / down / clean）已完成。
代码交付（submit / merge / reclaim）与运维辅助（capacity / diagnose / gc）
已实现。详见 [docs/SPEC.md](docs/SPEC.md)。

## 许可证

MIT
