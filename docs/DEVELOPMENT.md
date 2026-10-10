# LNH-MusicTag 开发文档

> 版本：v1.4.6 · 仓库：github.com/lahuolou/lnh-musictag · 协议：MIT
> 本文面向开发者：介绍项目架构、模块职责、数据模型、API、插件扩展点与发布流程。

---

## 1. 项目概述

LNH-MusicTag 是一个**自托管音乐库标签管理工具**：扫描本地音乐目录，读写音频标签（标题/艺术家/专辑/封面/歌词等），多源刮削补全元数据，智能去重，并支持格式转换、乱码修复、简繁体转换、音频内容识别等功能。**只提供 Docker 端**，所有操作通过 Web 后台完成（账号/密码/API Key 均存本地数据库）。

核心设计目标：

- **单二进制交付**：Go 后端通过 `go:embed` 内嵌 Vue3 构建产物，运行只有一个可执行文件。
- **无 CGo**：标签读写采用 go-taglib（WASM 封装 TagLib），SQLite 采用 modernc.org/sqlite（纯 Go），全静态编译，跨平台部署简单。
- **插件化扩展**：刮削源 / 工具 / 下载器 / 服务协议统一走插件注册表，后续新能力无需改页面。
- **后台化任务**：扫描、批量刮削、转换等全部异步执行，进度可视、刷新/换设备可恢复。

### 1.1 技术栈

| 层 | 技术 | 说明 |
|----|------|------|
| 后端 | Go 1.27（标准库 net/http + ServeMux 路由） | 无第三方 Web 框架 |
| 标签读写 | github.com/sentriz/go-taglib（WASM） | 免 CGo，支持多值标签 |
| 音频指纹 | chromaprint（fpcalc 子进程） | 音频内容识别 |
| 简繁转换 | OpenCC（zhscript 封装） | 简繁互转 |
| 存储 | modernc.org/sqlite（纯 Go SQLite） | 配置 + 曲目统一入库 |
| 前端 | Vue 3 + Vite | 无 UI 框架，自绘样式 |
| 部署 | Docker 多阶段构建 → ghcr.io 镜像 | CI 自动构建推送 |
| 更新检查 | GitHub API + jsDelivr CDN + raw 文件 | 三源容错 |

### 1.2 版本规划

- 版本号语义化递增：v1.4.1 → v1.4.2 → … → v1.4.50 → **v1.5.0** → v1.5.1 …
- 每次发布必须：递增 `main.go` 的 `AppVersion`、更新根目录 `VERSION` 文件、创建对应 GitHub Release tag（tag 名带 `v` 前缀）。
- 更新检查依据 `VERSION` 文件与 Release tag，语义化逐段比较（`1.4.50 → 1.5.0` 正确判新）。

---

## 2. 系统架构

```
浏览器 (Vue3 SPA)
   │  HTTP /api/*  (Cookie 会话 + CSRF + 限流)
   ▼
Go 后端 (单二进制，端口 10248)
   ├── 会话鉴权 auth.go          ── 登录/登出/改密/会话 Cookie
   ├── 安全防护 security.go      ── CSRF / XSS / SQL 注入防护 / 输入净化
   ├── 插件框架 plugin.go        ── 刮削源/工具/下载/协议 统一注册表
   ├── 刮削引擎 internal/scrape  ── 各源实现 + MultiSource 调度（语言优先）
   ├── 标签读写 internal/taglibx ── go-taglib 封装
   ├── 任务队列 scrapejob.go     ── 批量任务（进度/恢复/结果统计）
   ├── 工具链 convert.go / fixencoding.go / zhscript.go / identify.go
   ├── 曲目存储 tracks_db.go     ── SQLite 持久化（重启不丢、增量落库）
   ├── 更新检查 version.go       ── 多源最新版本查询 + 三态结果
   └── 静态资源 static.go        ── index.html no-cache / 哈希资源 immutable
           │
           ├── SQLite (lnh.db)   ── settings 表（账号/Key/选项/插件配置）
           │                        tracks 表（曲目 JSON）
           └── 音乐目录 (/music) ── 读写标签/封面/歌词文件
```

### 2.1 请求链路

1. 公开首页 `/`：无需登录（品牌 + 搜索占位），登录入口弹层。
2. 登录后进入后台：顶栏（曲目数/新版本徽标/任务进度/语言/主题/列表/首页/齿轮菜单）。
3. 所有 `/api/*` 除 `login` 外均要求会话 Cookie，经 `csrfGuard` + `limitBody` 中间件。
4. 前端 `api.js` 统一封装 fetch，自动带 `X-Requested-With: LNH-MusicTag` 头（CSRF 校验需要）。

---

## 3. 目录结构

```
项目根
├── main.go              程序入口：路由注册、中间件、静态资源、服务启动
├── auth.go              会话与账号：登录/登出/改密/随机初始密码/会话 Cookie
├── security.go          安全：CSRF、XSS 净化、SQL 注入防护、敏感键校验
├── plugin.go            插件框架：注册表/快照/启停/配置持久化
├── plugins_api.go       插件注册与 API：刮削源注册、GET/POST /api/plugins
├── version.go           更新检查：多源查询、三态结果、语义化版本比较
├── static.go            静态缓存策略：index.html no-cache、assets immutable
├── scrapejob.go         批量任务引擎：进度、恢复、成功/失败/跳过统计
├── helpers.go           通用工具：writeJSON、路径安全、标签标准化等
├── tracks_db.go         曲目 SQLite 持久化：落库/加载/外部删除清理
├── convert.go           格式转换（ffmpeg 转码）
├── ffmpeg.go            FFmpeg 插件化：路径探测、状态、一键安装
├── fixencoding.go       乱码修复（GBK/UTF-8 错乱检测与重写）
├── identify.go          音频内容识别（fpcalc 指纹 + 库内匹配）
├── zhscript.go          简繁体转换（OpenCC）
├── version_test.go      版本比较单测
├── *.go *_test.go       其他测试
├── internal/
│   ├── store/config.go  SQLite 键值配置存储（settings 表）
│   ├── model/model.go   数据模型：Track、DuplicateGroup、音频扩展名
│   ├── taglibx/         go-taglib 标签读写封装
│   ├── dedup/           去重策略（哈希/格式/标签）
│   ├── finger/          Chromaprint 指纹封装
│   ├── audiofmt/        音频格式探测
│   └── scrape/          刮削源：sources.go + 各源实现
├── web/
│   ├── package.json / vite 配置
│   └── src/
│       ├── main.js      入口
│       ├── App.vue      根组件：首页/后台切换、顶栏、齿轮菜单、全局弹窗
│       ├── store.js     全局状态（reactive）+ 任务轮询 + 更新检查
│       ├── api.js       fetch 封装
│       ├── i18n.js      中英双语字典
│       ├── style.css    全局样式（明暗主题变量）
│       ├── views/       Home（公开首页）/ Login / Settings / Plugin
│       └── components/  MusicTree（左列表）/ EditPanel / BatchPanel /
│                         DedupPanel / FormatDialog / Toast / UpdateModal / AboutModal
├── Dockerfile          多阶段：Vue 构建 → Go 静态编译 → alpine 运行
├── docker-compose.yml  推荐部署（端口/卷/环境变量/FFmpeg 开关）
├── entrypoint.sh       LNH_INSTALL_FFMPEG=true 时启动前自动安装 ffmpeg
├── VERSION             当前版本号（更新检查用）
└── README.md           使用说明（中英双语）
```

---

## 4. 数据模型

### 4.1 settings 表（SQLite 键值）

| 键 | 说明 |
|----|------|
| `admin_user` / `admin_pass` | 后台账号 / 密码（bcrypt 哈希） |
| `session_secret` | 会话签名密钥（首启生成） |
| `acoustid_key` 等 | 各 API Key（Last.fm / Spotify ID+Secret / Discogs / Jamendo） |
| `ffmpeg_path` | 旧键：FFmpeg 路径（兼容迁移） |
| `plugins` | 插件启停与配置 JSON（全量持久化） |
| `sources_enabled` | 旧键：刮削源开关（兼容迁移） |

### 4.2 tracks 表

| 列 | 说明 |
|----|------|
| `id` | 稳定 ID = SHA256[:16] |
| `path` | 服务器绝对路径 |
| `data` | Track 完整 JSON |
| `seq` | 扫描顺序（重启后按序恢复，保持"先扫先排"） |

启动时加载全部曲目（文件已外部删除的自动丢弃），扫描/编辑/重命名/删除均同步落库。

### 4.3 Track 结构（model）

```go
type Track struct {
    ID, Path, FileName, Ext string
    Size int64
    SHA256 string          // 去重策略1：内容哈希
    Duration float64        // 秒
    Bitrate, SampleRate, Channels uint
    HasCover, HasLrcFile bool
    HasTrad bool            // 标签含繁体（OpenCC 检测，供"繁体"筛选）
    Garbled bool            // 标签疑似乱码（GBK/UTF-8 错乱）
    NeedsIdentify bool      // 无文件名与标签信息 → 需音频内容识别
    Tags map[string]string  // 扁平化标签（标准键）
}
```

---

## 5. 后端模块详解

### 5.1 入口与路由（main.go）

- `AppVersion` 常量 = 当前版本号（发布时必须递增）。
- 路由：公开 `POST /api/login`、`POST /api/logout`、`GET /api/me`；其余全部挂 `pm`（plugins mux），统一套 `requireAuth → csrfGuard → limitBody`。
- 静态资源：`go:embed` 内嵌 `web/dist`，`/api/*` 优先于静态。
- 完整路由表见第 8 节。

### 5.2 会话鉴权（auth.go）

- 登录成功 → 生成签名会话 Cookie（HttpOnly + SameSite），密钥 `session_secret` 首启随机生成。
- 首次启动无密码 → 自动生成随机初始密码并打印到容器日志；登录后在「设置」页修改（写库）。
- 不强制修改密码；「设置」页改密后直接生效。

### 5.3 安全防护（security.go）

全项目统一安全层：

| 威胁 | 防护 |
|------|------|
| CSRF | 自定义头 `X-Requested-With` 校验 + Cookie SameSite |
| XSS | 前端 Vue 默认转义；后端对写入标签做 `<` `>` `&` `"` `'` 净化 |
| SQL 注入 | 全部参数化查询（database/sql 占位符） |
| 路径穿越 | 所有文件操作先 `filepath.Clean` 并校验在音乐目录内 |
| 拖库 | 密码 bcrypt 哈希存储；会话密钥随机；API Key 不在前端明文回显 |
| 请求放大 | `limitBody` 限制请求体大小；爬取外部 API 均带超时与限流 |

### 5.4 标签读写（internal/taglibx）

- 封装 go-taglib（TagLib 的 WASM 移植，无 CGo）。
- 标准化标签键：`title / artist / album / albumartist / genre / year / track / lyrics / comment`。
- 支持多艺术家（`/` 与 `_` 拆分）、多值读取、封面（APIC）与歌词（USLT）写入。
- 写操作统一走 `refreshTrack`：更新内存 + 落库（并保持列表按扫描顺序）。

### 5.5 扫描（main.go /api/scan）

- 异步增量扫描：按目录遍历音频扩展名，边扫边推（`/api/scan/status` 轮询），**不等待全部完成**。
- 每个文件：读标签 → 探测格式（时长/码率/采样率）→ 计算 SHA256 → 检测乱码/繁体/待识别标记 → 落库。
- 文件名解析（当标签缺失时）：`艺术家 - 标题.后缀` 拆分；多后缀先按音频格式探测，`发如雪.mp3.flac → 发如雪.flac`。
- 去后缀默认执行（作用于文件名，非标签标题）。
- 暂停/继续：`POST /api/scan/pause`。
- 排序：扫描中按扫描顺序（seq）递增；扫描完成/暂停后可点表头按标题/艺术家 A-Z 0-9。

### 5.6 刮削引擎（internal/scrape）

- `MultiSource`：按源注册表调度，`auto` 自动按语言优选（中文 → 国内源，外文 → 国际源）；也可手动指定源。
- 源清单（全部插件化，见第 6 节）：网易云 / QQ / 酷狗 / 酷我 / 咪咕 / B站 / 汽水 / 波点 / 5sing / MusicBrainz / iTunes / ListenBrainz / Deezer / Last.fm / Spotify / Discogs / Jamendo。
- 每个源实现统一 `Source` 接口：`Search(query) → SearchLyrics(title, artist)`。
- 刮削字段：标题/艺术家/专辑/专辑艺术家/流派/年代/曲目号/歌词/封面，逐字段可选、智能跳过已有标签。
- 批量任务 `/api/scrape/batch`：后台执行，进度 + 成功/失败/跳过统计，刷新/换设备可恢复。

### 5.7 批量任务引擎（scrapejob.go）

统一任务模型（批量刮削 / 格式转换 / 乱码修复 / 简繁转换 / 音频识别共用）：

- 内存任务表（`jobs`），`GET /api/jobs/{id}` 查询进度；`GET /api/jobs` 列出进行中任务。
- 前端启动任务后存 `jobId`，登录/刷新后 `resumeJobs()` 重新挂载轮询——**后台任务跨设备持续执行**。
- 结果统计：成功 / 失败 / 跳过 / 总数，任务结束汇总显示。

### 5.8 工具链

| 模块 | 功能 | 说明 |
|------|------|------|
| convert.go | 格式转换 | 调 ffmpeg 转码（插件化定位，见 6.3） |
| fixencoding.go | 乱码修复 | 检测 GBK/UTF-8 错乱（mojibake），识别后重写为正确编码；支持批量 |
| zhscript.go | 简繁体转换 | OpenCC：默认繁体→简体，可切换方向；作用于标签文本 |
| identify.go | 音频内容识别 | fpcalc 生成 Chromaprint 指纹，与库内已标注曲目匹配，命中补全标签与歌词 |

---

## 6. 插件框架（核心扩展点）

### 6.1 插件模型

```go
type Plugin struct {
    Name        string            // 唯一标识：scrape.qq / tool.ffmpeg / download.* / protocol.*
    Label       string
    Kind        PluginKind        // scrape | tool | download | protocol
    Enabled     bool
    Builtin     bool              // 内置不可停用（auto）
    Installable bool              // 支持一键安装
    NeedsKey    bool              // 依赖外部 API Key（默认关闭）
    KeyField    string            // 对应 settings 库 Key 键名
    KeyConfigured bool            // 快照时计算
    Fields      []PluginField     // 前端渲染表单
    Config      map[string]string // 始终序列化（至少 {}）
}
```

- `plugin.go`：注册表（`registerPlugin` / `pluginsSnapshot` / `applyPluginState` / `loadPluginsFromDB` / `savePluginsToDB`）。
- `plugins_api.go`：启动时注册全部刮削源 + FFmpeg；`GET /api/plugins`（快照）、`POST /api/plugins`（启停/配置更新）。
- 免 Key 刮削源在 `loadPluginsFromDB` 中**强制开启**；需 Key 源默认关闭，配置 Key 后自动变为可开启。
- 持久化：全量写入 `settings.plugins` 键（JSON），并同步兼容旧键。

### 6.2 新增一个插件（示例）

```go
// 在 plugins_api.go 的初始化函数中加入：
registerPlugin(&Plugin{
    Name:   "download.example",
    Label:  "示例下载器",
    Kind:   PluginDownload,     // 或 PluginProtocol（Subsonic 等）
    Fields: []PluginField{{Key: "url", Label: "fUrl", Type: "text"}},
}, func(name string, enabled bool, cfg map[string]string) {
    // 启停/配置变更时的生效回调（如开关外部服务）
})
```

注册后前端**无需改动**自动渲染：开关 + 表单 + Key 徽标。下载 / 协议类型当前为预留扩展位（音源、下载器、自动换源、Subsonic 等插件后续接入）。

### 6.3 FFmpeg 插件化

- 镜像默认**不装** ffmpeg；`LNH_INSTALL_FFMPEG=true`（compose）时 `entrypoint.sh` 启动前 `apk add ffmpeg`。
- 三种启用方式：① compose 开关自动安装；② 插件页「一键安装」；③ 插件页填宿主机 ffmpeg 绝对路径。
- `GET /api/ffmpeg/status`：探测路径与版本；`POST /api/ffmpeg/install`：容器内安装。

---

## 7. 前端详解（Vue3）

### 7.1 状态与路由（store.js）

- `state`（reactive）：authed / view（home|app）/ activeTab（list|edit|batch|dedup）/ tracks / selected / scanStatus / batchJob / toolJob / version 等。
- 无 vue-router：`activeTab` + `view` 组合控制页面；hash 仅保留简单路由。
- 任务轮询：`resumeJobs()` 在登录/刷新后按 `jobId` 重新挂载进度轮询。
- `checkVersion(force)`：force（手动）→ `/api/version/check` 实时重查；自动 → `/api/version` 读缓存；三态反馈（见 8.9）。

### 7.2 页面结构

| 视图 | 说明 |
|------|------|
| `views/Home.vue` | 公开首页：品牌 + 搜索占位（未接入）+ 功能亮点 + 登录入口 |
| `views/Login.vue` | 登录弹层（成功 `emit('done')` + 接回任务 + 检查更新） |
| `App.vue` | 后台框架：顶栏（徽标/进度/语言/主题/列表/🏠/齿轮菜单）、合并页布局 |
| `components/MusicTree.vue` | 左侧树形列表：目录 → 曲目名（显示文件名去掉后缀）；表头冻结；筛选（全部/乱码/繁体/歌词/海报/无艺术家/待识别）；输入即搜歌名 |
| `components/EditPanel.vue` | 右侧编辑：载入原标签 + 文件名回填、保存后按"艺术家-标题"重命名、刮削/歌词/封面 |
| `components/BatchPanel.vue` | 批量：补全字段勾选 + 智能跳过；音频识别 / 格式转换 / 乱码修复 / 简繁转换 |
| `components/DedupPanel.vue` | 去重：哈希 / 格式 / 标签分组；同名多格式保留音质最佳（可选删除） |
| `views/Settings.vue` | 设置：改密 + 数据库配置（API Key） |
| `views/Plugin.vue` | 插件页：四类插件开关 + Key 徽标 + FFmpeg 状态/路径/安装 |
| 弹窗组件 | `FormatDialog`（转换）、`UpdateModal`（更新提醒）、`AboutModal`（关于卡片）、`Toast` |

### 7.3 国际化（i18n.js）

- `lang`（zh/en）+ `t(key)` 字典查找；所有用户可见文案走 i18n，新增文案需补中英两处。

### 7.4 样式（style.css）

- CSS 变量主题：`--bg / --panel / --text / --accent` 等，`data-theme="dark|light"` 切换，默认跟随系统。
- 移动端适配：列表页小屏单列、横屏不横向溢出。

---

## 8. API 参考

> 除标注外均需登录会话。请求头 `X-Requested-With: LNH-MusicTag`。

### 8.1 会话

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/login` | 登录（body: user/pass），下发会话 Cookie |
| POST | `/api/logout` | 登出 |
| GET | `/api/me` | 当前会话用户（刷新恢复登录态用） |
| POST | `/api/change-password` | 修改密码 |

### 8.2 扫描与曲目

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/scan` | 开始扫描（body: dir） |
| POST | `/api/scan/pause` | 暂停/继续 |
| GET | `/api/scan/status` | 扫描进度（running/paused/added/total/done/skipped） |
| GET | `/api/tracks` | 曲目列表（按 seq） |
| POST | `/api/tracks/delete` | 批量删除选中文件 |
| POST | `/api/tracks/{id}/tags` | 写标签（含重命名联动） |
| GET | `/api/tracks/{id}/cover` | 取内嵌封面 |
| POST | `/api/tracks/{id}/cover` | 写封面 |
| GET | `/api/tracks/{id}/lrcfile` | 外挂歌词文件 |

### 8.3 刮削

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/scrape/sources` | 源列表与开关 |
| GET | `/api/scrape/search` | 指定源搜索（query + source） |
| POST | `/api/tracks/{id}/scrape` | 单曲刮削（source + 字段勾选） |
| POST | `/api/scrape/batch` | 批量刮削（ids + source + 字段），返回 jobId |
| GET | `/api/lyrics` | 歌词搜索/获取 |

### 8.4 批量任务

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/jobs/{id}` | 任务进度（done/total/ok/fail/skip） |
| GET | `/api/jobs` | 进行中任务列表（刷新/换设备恢复） |
| POST | `/api/convert` | 格式转换（target 格式） |
| POST | `/api/fix-encoding` | 乱码修复 |
| POST | `/api/convert-script` | 简繁转换（to: simp/trad） |
| POST | `/api/identify` | 音频内容识别 |
| POST | `/api/set-chorus` | 艺术家 ≥3 时改"合唱" |

### 8.5 去重

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/duplicates` | 重复分组（hash/format/tags；format 组含 keepId 音质最佳） |
| POST | `/api/duplicates/remove` | 删除组内选中文件 |

### 8.6 设置与插件

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/settings` | 设置（Key 脱敏返回） |
| POST | `/api/settings` | 保存设置/Key |
| GET | `/api/plugins` | 插件快照（含 config 恒非空、KeyConfigured） |
| POST | `/api/plugins` | 更新启停/配置（body: {name: {enabled?, config?}}） |
| GET | `/api/ffmpeg/status` | FFmpeg 路径与版本 |
| POST | `/api/ffmpeg/install` | 容器内一键安装 |

### 8.7 更新检查

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/version` | 缓存快照（current/latest/url/status） |
| GET | `/api/version/check` | 强制实时重查（手动检查用） |

响应 `status`：`ok`（成功取得 latest）/ `fail`（全部源失败）/ `unknown`（未检查）。前端据此区分"有新版 / 已是最新 / 检查失败"三种提示。

---

## 9. 部署

### 9.1 docker compose（推荐，含 1Panel）

```yaml
services:
  lnh-musictag:
    image: ghcr.io/lahuolou/lnh-musictag:latest
    container_name: lnh-musictag
    ports:
      - "10248:10248"
    volumes:
      - /mnt/sata3-1/mp3:/music        # 音乐目录（需读写）
      - lnh_config:/config             # 配置卷（密码/Key/数据库）
    environment:
      - LNH_ADMIN_USER=admin           # 仅首次初始化
      - LNH_ADMIN_PASS=                # 留空则随机初始密码打日志
      - ACOUSTID_API_KEY=
      - LNH_INSTALL_FFMPEG=false       # true 时启动自动装 ffmpeg
    restart: unless-stopped
volumes:
  lnh_config:
```

### 9.2 关键环境变量

| 变量 | 作用 |
|------|------|
| `LNH_ADMIN_USER` / `LNH_ADMIN_PASS` | 首次启动初始化种子（之后改库） |
| `LNH_INSTALL_FFMPEG` | true/1/yes/on → entrypoint 启动前安装 ffmpeg |
| `LNH_CONFIG_DIR` | 非容器运行时的配置目录（默认 `/config`） |
| `ACOUSTID_API_KEY` 等 | 各 API Key 初始化种子 |

### 9.3 构建

```bash
cd web && npm ci && npm run build   # 前端产物 → web/dist
cd .. && go build -ldflags "-s -w" -o lnh-musictag .
# Docker（CI 已自动）：
docker build -t lnh-musictag .
```

---

## 10. 发布流程（CI/CD）

1. **改码**：项目根（唯一真源）修改代码；前端改完 `npm run build`。
2. **验证**：`go vet ./...` + `go test ./...` + `go build`；本地起服务用浏览器冒烟（登录/关键页/console 无错）。
3. **同步**：robocopy 到发布副本（唯一 `.git`），`git add -A && git commit`（提交信息含 bump 说明）。
4. **推送**：`git push origin main` → GitHub Actions 自动构建并推送 `ghcr.io/lahuolou/lnh-musictag:latest`。
5. **版本**：先递增 `main.go` 的 `AppVersion` 与 `VERSION` 文件再提交（否则更新检查检测不到新版）。
6. **Release**：`gh release create vX.Y.Z --notes-file ...`（tag 必须带 `v`）。
7. **打包**：重新打源码 zip（含 web/dist 与 Dockerfile/compose/entrypoint），校验 dist 完整后交付。

> 注意：`VERSION` 文件与 Release tag 是更新检查的数据源，**两者必须与 AppVersion 同步递增**，否则登录后的"检查更新"会提示最新或失败。

---

## 11. 更新检查机制

- **数据源**（任一成功即用）：
  1. GitHub Releases API（`/releases/latest`，最实时）；
  2. jsDelivr CDN 读 `VERSION` 文件（大陆可访问性好，CDN 缓存最长约 12h）；
  3. raw.githubusercontent 读 `VERSION` 文件。
- **三态**：`ok`（latest 有效）/ `fail`（全部源失败）/ `unknown`（未查）。查询失败**不再误报"已是最新"**。
- **前端**：登录后自动检查一次，有新版弹 `UpdateModal`（同一版本每会话提醒一次，防打扰）；手动「检查更新」强制实时重查，分别提示 有新版 / 已是最新 / 检查失败。
- **版本比较**：语义化数值逐段比较（`parseVersion`），正确处理 `1.4.50 → 1.5.0`、`1.10.0 > 1.9.0`。

---

## 12. 常见问题排查

| 现象 | 排查方向 |
|------|----------|
| 更新后界面还是旧的（无 🏠/主题切换） | 浏览器/代理缓存了旧 index.html：强刷一次；已加 `Cache-Control: no-cache` 根治；确认容器拉取的是 latest 镜像 |
| 插件页空白 | 多为旧版本 bug（config omitempty 导致前端 undefined 崩溃）；v1.4.6 已修复，更新镜像即可 |
| 刮削/转换任务换设备后看不到进度 | 前端登录后 `resumeJobs()` 按 jobId 恢复轮询；确认后端 `/api/jobs` 有任务 |
| 扫描慢 | 异步增量扫描默认开启；首次扫描全量计算哈希属正常耗时 |
| 乱码筛选不出 | 乱码判定基于标签编码检测（GBK/UTF-8 错乱）；正常中文不应被标记 |
| 源搜索失败（status 500 / TLS 证书） | 源接口上游变动或网络受限；多个源互为备份，auto 会自动跳过失败源 |
| 需 Key 源搜不到 | 插件页显示 🔑 未配置 → 去「设置 → 数据库配置」填 Key 后自动开启 |
| 容器内无 ffmpeg 无法转换 | compose 设 `LNH_INSTALL_FFMPEG=true` 或插件页「一键安装」或填宿主机路径 |

---

## 13. 测试

- `go test ./...`：核心逻辑单测（版本比较、去重、刮削源、乱码修复、指纹等）。
- 本地 UI 冒烟：`%TEMP%\lnh_ui\lnh.exe` 起服务（配置目录指向测试库），浏览器验证登录、列表、编辑、批量、插件页渲染与 console 错误。
- 接口验证：curl 登录拿 Cookie 后逐接口检查（含 CSRF 头）。

---

## 14. 安全说明

- 密码 bcrypt 哈希；会话 Cookie HttpOnly+SameSite。
- 所有 SQL 参数化；文件路径严格校验；标签文本 XSS 净化。
- 更新检查、刮削等出站请求均带超时与 UA，避免滥用第三方接口。
- 建议部署时：容器内仅挂载音乐目录与配置卷；公网访问时置于反向代理后并启用 HTTPS。
