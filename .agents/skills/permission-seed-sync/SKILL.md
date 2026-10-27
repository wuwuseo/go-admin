---
name: permission-seed-sync
description: 把前端 permission-seed.ts 的权限种子同步到 Go 后端 system_permission.go。当需要同步/更新/新增菜单、按钮、API 权限定义，修复前端能进菜单但接口 403、后端 seedData 缺少条目、或前后端权限 code 不一致时使用。触发词：同步权限种子、permission seed、权限定义同步、seedData、permission-seed.ts、权限缺失。
---

# Permission Seed Sync

把前端权限种子（TypeScript）同步到后端 Go 种子数据（`InitSystemPermissions` 的 `seedData`），保证前端菜单/按钮与后端 API 鉴权定义一致。

## 适用场景

- 前端 `permission-seed.ts` 新增或修改了权限条目，需要落到后端
- 前端看得到菜单、点进去接口 403（后端缺对应 `api` 权限）
- 后端 `seedData` 缺菜单/按钮定义，角色授权页面勾不到
- 排查权限 code 前后端不一致、重复 code、父子关系错乱

**不适用**：Casbin 策略（`casbin_rule`）调整、角色-权限绑定数据修复、Ent schema 变更（改 schema 需 `go generate ./internal/ent`）。

## 关键文件

| 角色 | 路径 | 说明 |
|---|---|---|
| 源 | `D:\www\admin-web\web\src\data\permission-seed.ts` | 导出 `permissionSeedData`，单文件约 2000 行 |
| 目标 | `internal/service/system_permission.go` | `SeedPermission` 结构体（L132）、`InitSystemPermissions`（L727）、`seedData` |
| 落库 | 同文件 `InitSeedPermissions`（L617） | 按 `(Domain, Code)` 判断新增/更新 |

当前基线：`seedData` 约 201 条（menu/api/button 混合），最大 ID 205。同步前后用条数与 type 分布对比，能快速发现漏改。

## 执行步骤

1. **读源文件**：解析 `permissionSeedData` 数组，逐条拿到 `id / domain / type / name / code / path / method / menu_path / icon / component / parent_id / sort_order / is_common / status / description`。用 `Grep` 统计条数与各 `type` 数量（menu/api/button），作为后续核对基线。
2. **读目标文件**：定位 `InitSystemPermissions`，注意 `seedData` **不是单个字面量**，而是「基础字面量（L730–885）+ 若干 `seedData = append(...)` 块（如 L887 验证码、L926 用户管理/日志/队列、L973 幻灯片）」的组合，追加块都带业务注释说明来源。
3. **对比差异**：以 `code` 为业务主键（后端 upsert 就按 `Domain+Code`），产出三类清单：①源有后端无 → 需新增；②两边都有但字段不一致（path/method/parent_id/sort_order/status）→ 需修正；③后端有源无 → **默认保留**（可能是后端特有权限，如内部 queue-job 能力项），删除需用户确认。
4. **确定落点**：新增条目按业务模块归入对应块；新模块则在末尾追加独立 `seedData = append(seedData, ...)` 块，并在块上方写一行中文注释说明模块与引入原因（沿用现有风格）。
5. **写入并格式化**：改完执行 `gofmt -w internal/service/system_permission.go`。
6. **校验**：见下方「校验」。

## 字段映射

| TypeScript | Go `SeedPermission` | 转换规则 |
|---|---|---|
| `id` | `ID` | int，必须全局唯一（见「硬性约束」） |
| `domain` | `Domain` | 可照抄；末尾循环会用 `configuredPermissionDomain()` 统一覆盖 |
| `type` | `Type` | string，仅 `menu` / `api` / `button` |
| `name` | `Name` | **中文展示名**，前端常是英文标识（`dashboard`/`Workplace`），不要直接覆盖 |
| `code` | `Code` | 业务主键，前后端必须完全一致 |
| `path` | `Path` | 仅 `api` 用，完整路径含 `/api/v1/...` |
| `method` | `Method` | 仅 `api` 用，大写 `GET/POST/PUT/DELETE` |
| `menu_path` | `MenuPath` | 仅 `menu` 用，前端路由路径 |
| `icon` | `Icon` | 仅 `menu` 用，缺失时留空 |
| `component` | `Component` | 仅 `menu` 用，前端组件相对路径 |
| `parent_id` | `ParentID` | `null` → `0`（顶级），其余同值 |
| `sort_order` | `SortOrder` | 缺省 `0` |
| `is_common` | `IsCommon` | 缺省 `false` |
| `status` | `Status` | **int8**（`1` 启用 / `0` 禁用），注意字面量不要写成 `true/false` |
| `description` | `Description` | 中文描述，缺失时按「XX页面/XX API/XX按钮」补 |

## 硬性约束

- **ID 必须唯一**。`InitSeedPermissions` 先建 `seedByID` 映射，**后出现的重复 ID 覆盖前者**，会让 `ParentID` 指向错误记录。历史文件已存在 `145–150` 这 6 个 ID 被「OAuth 配置 API」与「验证码管理」两组重复占用（L876–881 vs L889–917）；因落库键是 `(Domain, Code)`，数据本身没错，但父节点解析有隐患。新增条目取**当前最大 ID + 1**（现为 205 之后），不要复用空洞 ID。
- **code 必须唯一**。upsert 键是 `(Domain, Code)`，重复 code 会互相覆盖，表现为「新增的权限把旧权限改掉了」。
- **不要照搬前端英文 `name`**：直接覆盖会让后端权限列表显示成英文标识，破坏已有中文界面。仅在条目为新增时才由前端语义生成中文名。
- **不要手工改生成的 Ent 代码**（`internal/ent/` 下除 `schema/`）。
- 保留原有 `// 中文注释`，它们记录了权限为什么存在（如 `S2-5：此前无任何角色可被授予用户管理入口`），删掉会丢失决策依据。
- 单条 SeedPermission 控制在单行或统一多行格式，与所在块风格保持一致，避免整块重排产生巨大 diff。

## 校验

```bash
# 语法与编译
gofmt -l internal/service/system_permission.go   # 应无输出
go build ./...                                    # 构建通过

# 权限相关单测
go test ./internal/service -run 'Permission|Seed' -count=1

# 重复 ID 自查（应无新增；已知历史遗留 145-150 属既有问题，勿盲目改动）
grep -oE '(^|[[:space:]{,])ID: [0-9]+' internal/service/system_permission.go \
  | grep -oE '[0-9]+$' | sort -n | uniq -d

# 重复 code 自查（结果应为空）
grep -oE 'Code: "[^"]+"' internal/service/system_permission.go | sort | uniq -d
```

改动涉及新路由时，还需确认 Casbin 策略与系统权限种子一致（`internal/service/permission*.go`、`internal/middleware/`）。

## 常见坑

- 只改了基础字面量、漏了后面的 `append` 块，导致验证码/用户管理/队列/幻灯片权限没更新。
- 把 `Status` 写成 `true`：`Status` 是 `int8`，编译报错或语义错。
- `parent_id` 指向了本次未新增也不存在的 ID：`resolveSeedParentID` 找不到会退化处理，表现为菜单挂到顶级或挂错，需要检查父 ID 是否在本文件内定义。
- 前端 `domain` 与 `config/config.yml` 里的权限域不一致：所有种子最终被 `configuredPermissionDomain()` 覆盖，按实际运行域核对即可，不要手动硬编码别的域。
- 后端存在但前端没有的权限（内部能力项）被误删，会让对应接口永久 403。
- 自查脚本写 `grep 'ID: [0-9]+'` 会把 `ParentID: 10` 也算成 `ID: 10`，产生一堆假重复；必须带左边界匹配。

## 调用示例

```
同步权限种子：把 D:\www\admin-web\web\src\data\permission-seed.ts 的权限定义同步到 internal/service/system_permission.go，只报告差异清单，确认后再改。
```
