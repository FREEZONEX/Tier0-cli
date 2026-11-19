# AGENTS.md

<!-- tier0-branch-lifecycle:begin -->
## 分支生命周期统一规则

- 同一仓库、同一交付目标优先复用已有功能分支、工作树和 PR/MR；创建前先检查现有记录。fork 会话、重试、评审修补、压测和阶段验证不构成新建远端分支或 PR 的理由；临时验证工作树默认只留本地。Cloud 的 dev 提测、release-ali 发版线遵循 Tier0 main：从 origin/release-ali 创建干净功能分支，合入 dev 提测后，以同一功能分支直接向 release-ali 提 PR，不为发版再建中转分支；此条优先于该发布线的历史子仓规则。其他发布线保持目标仓已确认的基线。
- **已合入 dev/develop 不等于已发版**。未完成发版或验收、无可信发布标记、仍有独有提交的分支必须保留；不能以名称含 final、PR 已关闭或时间较早推断完成。版本型仓库须核对正式发布标签；非版本型文档仓库仅在策略明确允许时以维护主线为完成依据。
- 保留默认/受保护/长期维护/备份分支、进行中的 PR/MR（含 Draft）、任何本地工作树占用的分支（无论干净或有修改）、未归档会话引用的分支及显式保留名单。跨仓库查询、会话状态、发布证据或提交包含关系无法核实时，一律保留并报告原因。
- 合并前在 Tier0 元仓库运行 `bash shell/cleanup-branches.sh settings --repo <仓库ID> --summary`。GitHub 的合并自动删分支会绕过上述保留条件，必须由管理员关闭后再自动合并需要保留源分支的 PR；Codeup 合并时不得删除仍需保留的源分支。
- 任务完成时运行 `bash shell/cleanup-branches.sh plan --repo <仓库ID> --summary --output <计划.json>`。仅在清理已获授权后执行 `bash shell/cleanup-branches.sh apply --apply --plan <计划.json> --audit <新审计.json>`；执行前重查 PR/MR、发布标记、会话、工作树与远端 SHA，条件变化即保留。
- 删除前保存分支名、精确 SHA、合并/发布依据和本地恢复引用，删除采用 SHA lease，随后远端回读。禁止按前缀或天数直接批量删除，禁止为了清理回退或覆盖未提交改动。找不到统一脚本时记录待清理，不能改用缺少保护的临时删除脚本。
- 规则源为 Tier0 根 `AGENTS.md` 的本节，策略为 `shell/branch-lifecycle.json`；用 `python3 shell/sync_branch_policy.py --check` 检查各仓库入口一致性，使用 `--apply` 同步。该规则不授权主动创建会话、推送代码、合并 PR 或部署。
<!-- tier0-branch-lifecycle:end -->
