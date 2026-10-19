---
name: develop
description: 为一个任务开 dock（new + up）、在里面干活、保持隔离。用于用户要求在隔离环境里做任务。
---

# 技能：在 dock 里开发

目标：每个任务都在自己的 dock 里跑——自己的 worktree、端口段、栈——并行任务互不干扰。

## 1. 开工前

- 确认配置已发现：`.agentdock/config.json` 必须存在且非空。若缺失或还是占位，停下先跑 `bootstrap` 技能——**不要**用猜的服务名单往下走。
- 跟用户**确认一次** dock 名（`<slug>`）。
- 若 `config.auth.detected` 为 true，说明项目有账号登录。**开 dock 前**先问用户这个 dock 的管理员账号密码怎么设，绝不自造凭据。

## 2. 开 dock

```sh
agentdock new --admin-user <用户名> --admin-password <密码> <slug>
```

或仅在用户明确接受默认值时：

```sh
agentdock new --use-default-auth <slug>   # admin / admin&798Ad
```

引擎会建 worktree、从槽位模型分配端口段、写 override（容器名 `songji-<slug>-*`）和这个 dock 自己的 `.env`（仓库 `.env` 的副本，管理员用户名/密码键用上面的值覆盖）。若 `config.auth.detected` 为 true 而你没传凭据，引擎会拒绝——把拒绝如实上报，不要绕过。

## 3. 起栈

```sh
agentdock up
```

引擎会先校验端口和变量，拒绝起一个残缺的栈。80/443 绝不给 dock 用。

## 4. 干活

**只在当前这个 dock 的 worktree 里干活**，绝不改主 checkout。把兄弟 worktree 和别的 dock 当作不存在：不读、不引用、不让别的 dock 的状态阻塞或带偏当前任务——冲突留给 `merge`/`submit` 时处理，绝不在任务中途解决。若端口意外被占，**不要**自己换一个——问引擎（`agentdock new`/槽位模型），保证台账真实。

## 5. 回收

完成后跑 `agentdock down` 再 `agentdock clean`（见 `reclaim` 技能）。绝不把 dock 留着「以防万一」——用完即收。
