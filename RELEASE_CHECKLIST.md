# Release Checklist

## 前置条件

- [ ] `main` 分支上所有 CI 通过
- [ ] `go test ./... -race -count=1 -timeout=12m` 全绿
- [ ] `go build ./...` 通过
- [ ] `gofmt -l .` 输出为空
- [ ] `golangci-lint run ./...` 通过

## 版本发布流程

1. **更新版本号**
   - [ ] 修改 `version.go` 中的版本常量
   - [ ] 在 `CHANGELOG.md` 顶部添加新版本章节，并将 Unreleased 内容移入

2. **更新文档**
   - [ ] 检查 `README.md` 中的版本徽章与示例
   - [ ] 检查 `MIGRATION.md` 是否需要更新
   - [ ] 检查 `docs/` 是否需要更新

3. **创建 Release PR**
   - [ ] 分支名：`release/v2.x.x`
   - [ ] PR 标题：`chore(release): prepare v2.x.x`
   - [ ] 通过 CI 后合并到 `main`

4. **打 Tag**
   ```bash
   git checkout main
   git pull origin main
   git tag -a v2.x.x -m "Release v2.x.x"
   git push origin v2.x.x
   ```

5. **创建 GitHub Release**
   - [ ] 使用 Tag 创建 Release
   - [ ] 标题：`AgentScope.Go v2.x.x`
   - [ ] 内容：复制 `CHANGELOG.md` 对应章节
   - [ ] 勾选 "Set as the latest release"

6. **发布后续**
   - [ ] 在 Discord/钉钉/微信群公告
   - [ ] 更新 `docs/` 站点版本
   - [ ] 监控 Issue 反馈，准备 patch release

## 紧急 Patch Release

对于安全修复或严重 bug：

1. 从对应 tag 切出 `hotfix/v2.x.x+1` 分支
2. 修复并补充回归测试
3. 走 Release PR → Tag → Release 流程
4. 在 Release Note 中标注 "Security fix" 或 "Critical bug fix"

## 回滚策略

如发布后发现严重问题：

1. 立即在 GitHub Release 中标记为 `Pre-release` 或添加警告
2. 在 README 和文档站点顶部添加警告横幅
3. 24 小时内发布 patch 或回滚指南
