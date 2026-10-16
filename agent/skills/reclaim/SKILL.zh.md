---
name: reclaim
description: 任务结束回收 dock。用于合并/放弃任务后清理环境。
---

# 技能：回收

目标：任务结束后拆掉 dock，不留残留。

## 流程

1. `agentdock down` 停栈。
2. `agentdock clean` 拆 worktree + 容器 + 端口段。
3. 确认没有残留容器和端口占用。
