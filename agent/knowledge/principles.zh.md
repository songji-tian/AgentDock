# 原则（中文）

1. **永不猜**：缺事实就查证，查不到就拒绝。
2. **discover 是唯一的事实来源**：不要用 LLM 读 YAML，用 `agentdock discover`。
3. **init 是空的**：服务名单永远来自 `--from`，引擎不内置任何业务默认。
4. **fail-closed**：占位/未发现的配置，引擎拒绝而不是照做。
5. **端口排除 80/443**：它们是主栈专用。
6. **一个 worktree 一个任务**：在 dock 里只在它的 worktree 干活，兄弟 worktree 和主 checkout 都当作看不见。冲突推迟到合并时处理，不在开发中途解决。
