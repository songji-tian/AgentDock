---
name: bootstrap
description: 把一个从没用过 AgentDock 的仓库接入进来。发现栈、和用户核对一次、写 .agentdock/config.json、验证。用于用户说"装 AgentDock""加隔离环境"或首次运行。
---

# 技能：接入一个仓库

目标：把一个普通仓库变成 AgentDock 能开 dock 的仓库。

## 1. 发现（discover）

跑 `agentdock discover`，拿到事实清单：每个服务的名字、端口、容器名、必需变量。
**不要**自己用眼睛读 compose——那样会漏端口（历史上漏过 443、3001）。
用 `discover` 的确定性输出。

## 2. 核对（一次）

把清单给用户看，核对两件事：
- 服务/端口清单对不对（有没有漏的服务）。
- 这个"工作站"叫什么名（后续 dock 从它派生端口和容器名）。

只问一次，问聚焦的问题。

## 3. 写配置

把确认后的清单交给引擎：`agentdock init --from <清单>`。服务名单**永远来自
`--from`**，绝不用内置默认值。`agentdock init` 本身是空的。

## 4. 验证

证明它真的能跑：`agentdock new` 开一个 dock → `agentdock up` 起栈 → 确认服务
在分配的端口可达、没占用 80/443 → `agentdock clean` 拆掉。任何一步被引擎拒绝，
读拒绝原因、修配置，不要绕过它。
