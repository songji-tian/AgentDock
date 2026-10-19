---
name: diagnose
description: 环境起不来时定位问题。用于 dock 起不来、端口冲突、容器异常时。
---

# 技能：诊断

目标：环境起不来时，定位是配置错、端口冲突还是容器问题。

## 顺序

1. 读引擎的拒绝信息（它拒绝一定有原因，先读原因）。
2. 先跑只读检查：

   ```sh
   agentdock capacity          # 端口台账 + 剩余空间
   agentdock diagnose [SLUG]   # 单个 dock 的一致性检查（只读，绝不改动）
   ```

   `diagnose` 输出 `ok` / `warn` / `err` 三级：worktree 在不在、compose
   override 在不在、`.env` 在不在、台账状态。`err` 是阻塞项；`warn` 表示
   「下次 `up` 会自动补上」。

3. 读 `.agentdock/state.json` 核对这个 dock 占的端口/slug/worktree。
4. `docker compose ps` / `docker compose logs <service>` 看实际在跑什么、为什么没起来。
5. 检查端口段是否撞上主栈（尤其 80/443）、必需变量是否给了值。
6. 修根因，不要绕过校验。拿证据汇报，需要决策时只问一个聚焦问题。
