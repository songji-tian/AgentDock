---
name: finish
description: 任务收尾：交付成果（submit）、合回主线（merge）、回收环境（reclaim）。用于 dock 里的活干完后。
---

# 技能：任务收尾

目标：把 dock 里干完的活交付出去，再清理干净——不留运行中的栈，不留占用的端口。

## 0. 先确认收尾环境（agent 驱动）

`submit` 之前，先问用户成果要交到哪：

- **local（本地）**——只提交，不推送（默认）。
- **个人 GitHub**——推送到用户自己的 GitHub。**默认私有**，要公开就加
  `--visibility public`。远程仓库不存在会自动用 `gh repo create` 建。
- 公司 GitLab / 云端 / 开源 GitHub——**还没接**；动手前先跟用户确认。

绝不猜环境。另外，中间产物（比如 discover 导出的 JSON）要放在**仓库目录
外面**——多一个未跟踪文件会让主 checkout 变脏，卡住 `merge --mode direct`。

## 1. 交付

```sh
agentdock submit SLUG                        # 只提交（本地）
agentdock submit --target github SLUG        # 提交 + 推送（私有）
agentdock submit --target github --visibility public SLUG
```

`submit` 在没有改动、或 worktree 缺失时会拒绝。

## 2. 合回主线

```sh
agentdock merge --mode direct SLUG           # 本地合到 main
agentdock merge --mode pr SLUG               # 开 PR（分支必须先推送）
```

`direct` 在主 checkout 脏、分支不对、或冲突时都会拒绝——冲突靠手工解决，
绝不自动合并。

## 3. 回收

```sh
agentdock reclaim SLUG                       # 一条命令停栈 + 拆
```

细节和 `gc` 见 `reclaim` 技能。
