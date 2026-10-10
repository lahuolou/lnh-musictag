# LNH-MusicTag 插件开发指南

本文档说明 LNH-MusicTag 的插件框架：如何声明一个插件、如何导入第三方插件、
以及如何通过源码扩展点接入真正的执行逻辑。

## 一、插件是什么

插件是一份 **JSON 声明**（名称、展示名、类型、版本、配置字段），注册进统一的
插件注册表后，前端「插件」页会自动渲染（启停开关 + 配置表单），启停与配置
持久化到 SQLite（settings 表 `plugins` 键），重启后自动恢复。

插件分为四类（`kind`）：

| kind       | 说明                                   | 示例                        |
|------------|----------------------------------------|-----------------------------|
| `scrape`   | 刮削源 / 音源（元数据、歌词、封面）      | `scrape.qq`、`scrape.netease` |
| `tool`     | 工具                                   | `tool.ffmpeg`               |
| `download` | 下载器（预留）                          | —                           |
| `protocol` | 服务端协议（Subsonic 等，预留）          | —                           |

## 二、插件声明格式

```json
{
  "name": "scrape.mysource",
  "label": "我的源",
  "kind": "scrape",
  "version": "1.0.0",
  "hint": "ffHint",
  "fields": [
    { "key": "api_url", "label": "API 地址", "type": "text", "placeholder": "https://..." },
    { "key": "token", "label": "Token", "type": "password" }
  ]
}
```

字段说明：

| 字段      | 必填 | 说明                                                         |
|-----------|------|--------------------------------------------------------------|
| `name`    | ✅   | 全局唯一标识，如 `scrape.xxx` / `tool.xxx`。不得与内置/已导入插件重名。 |
| `label`   | ✅   | 展示名（可含中文）                                           |
| `kind`    | ✅   | `scrape` / `tool` / `download` / `protocol`                  |
| `version` |      | 插件版本号                                                   |
| `hint`    |      | 提示文案（i18n key 或纯文本均可）                             |
| `fields`  |      | 配置字段数组，前端据此渲染表单                                 |

配置字段（`fields[]`）：

| 子字段        | 说明                        |
|---------------|-----------------------------|
| `key`         | 配置键名，存入插件 config    |
| `label`       | 表单标签（i18n key 或纯文本） |
| `type`        | `text` / `password` / `bool` |
| `placeholder` | 输入框占位                  |

## 三、导入第三方插件

在「插件」页顶部「导入第三方插件」区域粘贴 JSON 后点导入，三种格式均可：

- 单个对象（见上节示例）
- `{"plugins": [ ... ]}` 包裹的数组
- 裸数组 `[ {...}, {...} ]`

导入成功即注册进插件列表（标记 `ext`，默认开启），并持久化到数据库，
重启后保留。导入校验规则：

- `name` / `label` 非空；
- `kind` 必须是四种类型之一；
- `name` 不得与内置或已导入插件冲突（防止覆盖内置插件）。

对应接口：`POST /api/plugins/import`（body 即 JSON 声明，上限 256KB）。

## 四、接入执行逻辑（源码扩展点）

当前版本导入的第三方插件先以「声明 + 启停 + 配置管理」方式接入：开关与配置
可见、可持久化，但**具体执行逻辑**需通过源码扩展点接入，重新构建镜像后生效。
以下扩展点位于 Go 后端：

1. **刮削源（scrape）**：在 `internal/scrape/` 下实现 `Source` 接口
   （`Search(ctx, query) ([]Result, error)` 等），然后在
   `registerBuiltinPlugins`（`plugins_api.go`）追加注册：
   ```go
   registerPlugin(&Plugin{
       Name: "scrape.mysource", Label: "我的源", Kind: PluginScrape,
   }, func(n string, on bool, _ map[string]string) {
       ms.SetEnabled("mysource", on) // 联动刮削源启停
   })
   ```
2. **工具（tool）**：实现 `applyFn` 生效回调，读取 `plugin.Config` 执行工具逻辑。
3. **下载器 / 协议（download / protocol）**：预留类型，按同一套 `registerPlugin`
   接入，前端无需改动。

## 五、API 一览

| 方法 | 路径                    | 说明                                   |
|------|-------------------------|----------------------------------------|
| GET  | `/api/plugins`          | 列出全部插件（含启停 / 配置 / Key 状态） |
| POST | `/api/plugins`          | 批量启停 / 保存配置，持久化并即时生效     |
| POST | `/api/plugins/import`   | 导入第三方插件声明                       |
| GET  | `/api/ffmpeg/status`    | FFmpeg 可用状态与版本                   |
| POST | `/api/ffmpeg/install`   | 容器内一键安装 FFmpeg                   |
