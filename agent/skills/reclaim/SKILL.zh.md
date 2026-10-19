---
name: reclaim
description: 任务结束回收 dock。用于合并/放弃任务后清理环境。
---

# 技能：回收

目标：任务结束后拆掉 dock，不留残留。

## 1. 一条命令回收

```sh
agentdock reclaim SLUG
```

会先停栈（如果还起者）再拆，一条命令搞定。单独的 `clean` 在栈还运行时
会拒绝——这个拒绝是保护，不是麻烦。

## 2. 批量垃圾回收

```sh
agentdock gc          # 只列出垃圾 dock（dry run，安全）
agentdock gc --yes    # 真正回收它们
```

`gc` 只回收它**能证明没在跑**的 dock。凡是判定不了的（docker 不可用、
没有 compose 文件）一律留着并单独报告——绝不靠猜就拆。

## 3. 确认无残留

- `agentdock capacity` — 台账不再占用这个 dock 的端口段。
- `agentdock diagnose SLUG` — 应报告该 dock 已不存在。
- 没有 `<project>-<slug>-*` 前缀的残留容器。

还有残留就先读 `diagnose` 输出，再继续。
