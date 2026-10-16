# 把 AgentDock 注册进你的 coding agent

AgentDock **不是你要 import 的库**——它是你**装进 coding agent 上下文里的
一个 agent**。"注册"的意思是让你的运行时能读到 `agent/AGENT.md`（身份 + 规则）
和 `agent/skills/`（流程）。

## 思路

```
你的 coding agent（Codex / Claude Code / Cursor / OpenCode / …）
        │ 读
        ▼
  agent/AGENT.md         身份 + 铁律（"永不猜"）
  agent/skills/*         流程（bootstrap / develop / diagnose / reclaim）
        │ 驱动
        ▼
  agentdock 引擎         执行 + 校验
```

## 三步注册

```bash
# 1. 装引擎二进制
go install github.com/songji-tian/agentdock/cmd/agentdock@latest

# 2. 克隆 agent 半件到稳定位置
git clone git@github.com:songji-tian/AgentDock.git ~/.agentdock/

# 3. 在你的项目根放一行指针
```

指针内容（按你的运行时选文件）：

| 运行时 | 文件 | 内容 |
|---|---|---|
| Codex | `AGENTS.md` | `Before any environment/worktree/port/cleanup work, read ~/.agentdock/agent/AGENT.md and follow its skills.` |
| Claude Code | `CLAUDE.md` | 同上 |
| Cursor | `.cursor/rules/` | 同上 |

## 验证

问你的 agent："the one rule?" 它应回答：

> Never guess. Verify, or refuse.

答对即注册成功。之后你在项目里说一句 "set up AgentDock in this repo"，agent
会跑 bootstrap：`agentdock discover` 读你的栈 → 你核对一次服务/端口清单 →
agent 写 `.agentdock/config.json` → 冒烟验证。这就是全部接入流程。

> 如果你的运行时没有"skills"概念，`SKILL.md` 仍是可读说明书——把相关文件
> 贴进提示词即可。这套契约不依赖任何厂商特性。
