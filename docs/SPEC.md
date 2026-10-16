# AgentDock — 设计规格（SPEC）

> 状态：M0 起草，随里程碑更新。本文件是实现的唯一事实来源。

## 0. 产品定位

**为 AI Coding Agent 提供独立、可复现的并行开发环境，且由 Agent 驱动使用的一套系统。**

AgentDock 让多个 AI 编码助手能在同一个仓库上并行工作，各自拥有隔离的
Docker 开发栈（worktree + 独立端口段 + 独立容器），互不干扰、可复现、可回收。

## 1. 为什么重写：从 eff-agent-si 学到的教训

AgentDock 是 eff-agent-si 的干净重写。旧项目设计方向是对的，但实现有两个 bug，
直接导致"任何人拉下来都会用错"。这两个 bug 是本规格的约束来源。

### Bug 1 —— agent 驱动路径"不可发现"
- 现象：README 把"注册 agent"和"直接跑 CLI"两条路并列，`eff init` 跑完也不提示
  "真正的栈分析要由 agent 做"。
- 后果：使用者跳过 agent、直接敲 CLI，拿到一份写死的错误配置还不自知。
- **约束**：README 里 agent 注册必须是**唯一主路径**；任何第一次运行都要被明确告知
  "这是 agent 驱动，正确用法是 X"。

### Bug 2 —— `init` 不 fail-closed，静默猜
- 现象：`cmdInit` 直接 `config.Default(name)` 写死 4 个服务（api/web/db/redis），
  一个都不读仓库。这违反了旧项目 AGENT.md 自己的铁律 *"Never guess. Verify, or refuse."*
- 后果：名单配漏一个（如 nginx 443、飞书 3001）就起不来，且静默、无报错。
- **约束**：`init` 必须**空**（绝不写死服务名单）；`new`/`up` 遇到未发现/占位配置必须拒绝。

### 继承 worktree-env.sh 的成熟做法（保留优点）
- 容器名 `songji-$slug-*`（带任务名，天然隔离，不撞主栈）。
- 端口段整体平移（80→20xxx、443→20xxx 一起换）。
- 写 compose 前先做容量/格式检查。

## 2. 架构：两个半件

```
你的 coding agent（判断 / 流程）
      │ 读 agent/AGENT.md + agent/skills/（说明书：身份、铁律、流程）
      │ 调 agentdock 二进制
      ▼
agentdock 引擎（确定性执行 + 校验 / 拒绝）
```

**分工铁律：agent 提方案（判断），引擎验事实（读 + 校验 + 拒绝）。**

理由：LLM 会漏（这次就漏了 443、3001）。如果引擎对 agent 传入的东西照单全收，
漏一个就建出坏栈——等于回到"猜"。所以引擎必须在自己执行前校验并拒绝。

## 3. 核心原则

1. **Agent-first**：产品是 agent，引擎只是它的手。人把活交给 coding agent，
   agent 读说明书、做判断、调引擎。
2. **Never guess. Verify, or refuse.** 永不猜，要么验证，要么拒绝。
3. **init 空**：引擎绝不写死服务名单，业务参数（服务/端口/名称）全部由 agent 传入。
4. **discover**：引擎提供确定性解析 compose 的能力，把"事实清单"交给 agent，
   取代"LLM 用眼睛猜"。
5. **fail-closed**：`new`/`up` 遇到未发现/占位/不完整配置就拒绝并给指引。
6. **端口隔离**：slot 确定性分配（不随机挑），默认排除 80/443。
7. **一次核对**：开栈前和用户核对一次"服务/端口清单 + 工作站名"，之后端口自动派生。
8. **agent 注册 = 唯一主路径**：README 与首次运行都指向这条路径。

## 4. 命令面（核心闭环 A）

| 命令 | 作用 | fail-closed 行为 |
|---|---|---|
| `agentdock discover` | 确定性读 compose，输出事实清单（服务/端口/容器名/必需变量） | — |
| `agentdock init` | 建 `.agentdock/` 配置，**空**（不写死服务） | 不猜 |
| `agentdock init --from <清单>` | 接受 agent 传入的已确认清单写入配置 | 清单必须完整 |
| `agentdock new <slug>` | 建 worktree + 分配端口 + 写 override | 配置未发现→拒绝 |
| `agentdock up` | 起栈（先校验端口/变量） | 不完整→拒绝 |
| `agentdock down` | 停栈 | — |
| `agentdock clean` | 拆 worktree、回收端口 | 未停栈→拒绝 |

（v2 再补：submit / merge / reclaim / diagnose / capacity / gc）

## 5. 数据模型

- `.agentdock/config.json`：project、portBase、portStride、`services[]`（**来自 discover，
  非写死**，含 name / container_name / ports / requiredEnv）。
- `.agentdock/state.json`：每个 workstation 的台账（slug、worktree、分支、已分配端口）。

## 6. 里程碑与验收判据

| 里程碑 | 内容 | 验收判据 |
|---|---|---|
| **M0 骨架+SPEC** | 建仓、目录、README/AGENT.md/skills 骨架、本 SPEC | 任何人 clone 后第一次跑命令即被明确告知"agent 驱动 + 用法" |
| **M1 发现+接入** | `discover` + `init(空)` + `init --from` | `discover` 不漏列 Aisystem 的 6 服务/6 端口；`init` 不产生写死名单 |
| **M2 起栈** | `new`/`up`/`down`/`clean` + 端口分配 + container_name 重写 + .env 供给 | Aisystem relations 隔离栈 `up` 真正起来，不碰 80/443、不撞主栈 |
| **M3 收尾+退役** | 补 v2 命令 + 文档定稿 + 推 GitHub + 退役 eff-agent-si | 干净可 clone、可复现 |

## 7. 验收用例：Aisystem

- 6 个服务：api / web / db / redis / nginx-ssl(443) / feishu(3001)。
- 主栈正占 80 + 443；compose 里 6 处 `container_name: songji-*` 写死；
  `POSTGRES_PASSWORD` 为必需变量。
- **验收标准**：`agentdock` 在 Aisystem 起一套 relations 隔离栈，栈真实起来、
  不占用 80/443、容器名不撞主栈、用 `discover` 得出的完整清单驱动（而非写死）。
