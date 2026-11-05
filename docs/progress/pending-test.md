---
title: 待测试
description: 当前版本已实现但仍需人工验证的变更项
---

# 待测试

## 上游同步与历史图片恢复

- 原因与范围：同步上游至 `a166ac8`，保留 fork 的历史图片恢复和后台布局修复，继续排除测试源码。
- 合并决策：历史记录使用恢复后的图片列表生成 WebP 预览，同时保留图片地址、存储键及成功数量恢复逻辑。
- 验证：`go test ./...` 编译全部主服务包成功（仓库无测试源码）；`bun install --frozen-lockfile` 与 `bun run build` 成功，生产前端无依赖警告。测试源码和未合并文件检查均为零，待办清单无需调整。
- 部署结果：业务合并提交 `0a65781` 已推送并部署，服务器 Docker 构建成功，三个平台的 Bridge 产物存在。容器镜像与构建镜像一致；内外网 `/api/health` 返回 `ok`，登录页返回 200，SQLite `PRAGMA quick_check` 返回 `ok`，启动日志未发现错误。
- 回滚准备：数据和配置备份位于服务器 `/root/canvas-backups/sync-20261105-hMgtaP/data-and-config.tar.gz`，旧镜像保留为 `tigerowo-infinite-canvas:rollback-20261105`。
- 人工验证：重新进入含历史生成图片的页面，确认原图、预览和数量一致；新通道生图、生视频与语音需使用实际账号验证。
