---
name: develop
description: 为一个任务开 dock（new + up）、在里面干活、保持隔离。用于用户要求在隔离环境里做任务。
---

# 技能：在 dock 里开发

目标：每个任务都在自己的 dock 里跑——自己的 worktree、端口段、栈——并行任务互不干扰。

## 流程

1. `agentdock new`（给任务起个 slug）→ 得到专属 worktree + 端口段。
2. `agentdock up` → 起栈（引擎会校验；占位配置会被拒绝）。
3. 在分配的端口上干活。
4. 完成后 `agentdock clean` 回收。
