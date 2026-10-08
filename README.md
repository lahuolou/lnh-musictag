# LNH-MusicTag

纯 **Docker 部署**的自托管 Web 音乐标签工具（形态类似 Music Tag Web）：**标签读写 + 歌曲去重 + 多源元数据/歌词刮削 + 批量修正 + 异步扫描**，全部操作通过浏览器后台完成，无本地 CLI。

技术栈：**Go 后端（go-taglib WASM 封装 TagLib，无 CGo）+ 纯 Go SHA256 去重 + 11 个刮削源 + 歌词 + Vue3 多页面前端**，配置存 **SQLite**（不依赖环境变量）。

前端用 **Vue 3 + Vite**（比 React 运行时更轻），构建产物约 90KB JS，`go:embed` 进单个静态二进制，Docker 多阶段构建（node → Go → scratch），运行时无 node 依赖、不额外占内存。

## 核心能力

| 能力 | 实现 | 说明 |
|---|---|---|
| ① 标签读写 | `go.senan.xyz/taglib`（WASM，无 CGo，MP3/FLAC/M4A/OGG/WAV/WMA…），多值标签、内嵌封面 | 写标题/艺术家/专辑/专辑艺术家/流派/年份/曲目号/封面/歌词 |
| ② 异步扫描 | `/api/scan` 后台任务，**边扫边入库**；前端轮询 `/api/scan/status` 增量渲染曲目，不等全部扫完 | 扫描期间即见曲目 |
| ③ 歌曲去重 | SHA256 文件哈希（默认）+ 音频指纹（可插拔） | 去重页 |
| ④ 多源刮削 | **11 个源**：MusicBrainz / iTunes / 网易云 / QQ / 酷狗 / 酷我 / 咪咕 / 哔哩哔哩 / 汽水 / 波点 / 千千 | 搜索/应用，自动依次尝试 |
| ⑤ 补全元数据 | MusicBrainz release 详情回填**专辑艺术家、流派、年代、曲目号**；写入标签 | 批量/单曲刮削均可 |
| ⑥ 歌词 | 网易云 LRC + lrclib，自动选择 | 单曲获取/编辑/保存，批量「含歌词」 |
| ⑦ 批量 | 勾选曲目 → 批量刮削；**字段可选**（海报/歌名/艺术家/流派/年代/专辑艺术家/曲目号/歌词），**智能跳过**已有标签的曲目 | 批量页 |
| ⑧ 去后缀默认执行 | ① 文件重命名：扫描时按**真实编码格式**（读文件头魔数）去重复/错误音频后缀，`发如雪.mp3.flac → 发如雪.flac`、`发如雪.mp3.mp3 → 发如雪.mp3`；② 标签标题去后缀 `广岛之恋.mp3 → 广岛之恋` | 均默认开启，可在设置关闭 |
| ⑨ 配置存数据库 | 账号/密码/API Key/选项存 `SQLite`（`/config/lnh.db`），不再用环境变量 | 设置页读写 |
| ⑩ 列表搜索 | 左侧音乐列表实时搜索：标题/艺术家/专辑/文件名 | 本地过滤 |
| ⑪ 主题切换 | 黑/白天模式，现代扁平化风格，随偏好记忆 | 顶栏 🌙/☀️ |
| ⑫ 语言优选源 | 自动源按语言排序：中文→QQ/网易云等国内源优先，外文→iTunes/MusicBrainz 等国际源优先 | auto 源 |
| ⑬ 整合布局 | 单页：**整列竖排音乐列表**（目录→曲目，checkbox+图标+文件名，纵向滚动）+ **顶部功能 tab**（列表/编辑/批量/去重/设置） | 一体化界面 |
| ⑭ 格式转换 | ffmpeg 转码为 mp3/flac/m4a/ogg/opus/wav，保留标签，可选删除原文件 | 批量面板 · ffmpeg |
| ⑮ 乱码修复 | 自动还原 GBK/UTF-8 错读的中文标签（标题/艺术家/专辑等） | 批量面板 |
| ⑯ 简繁体转换 | 标签文本 简体⇄繁体（OpenCC 引擎） | 批量面板 |

## 快速开始（Docker）

前置：目标机已装 Docker（含 compose）。镜像为多阶段构建的静态二进制（scratch 运行层，约 10MB，带 CA 证书，构建时按目标机架构自动适配）。

```bash
# 1) 在项目目录构建并启动（或直接拉 GHCR 镜像）
cd /path/to/LNH-musictag
docker compose up -d

# 2) 编辑 docker-compose.yml，把 /mnt/sata3-1/mp3 改成你的真实音乐目录
```

浏览器访问 **http://<主机IP>:10248**：

1. **登录页**：账号默认 `admin`。**首次启动自动生成随机初始密码**，打印在容器日志（`docker compose logs`），登录后到「设置」改密码（存数据库）。
2. **列表（竖向整列）**：输入容器内音频目录（如 `/music`）→ 点「扫描」→ **边扫边出**；整列竖排列表（目录→曲目，checkbox+图标+文件名），点某首进「编辑」，可勾选多首。
3. **顶部功能 tab**：**列表 / 编辑 / 批量 / 去重 / 设置**——编辑标签/刮削/歌词；批量补全；去重检测；改密与配置。

> 账号、密码、AcoustID Key、默认目录、是否自动去后缀等均保存在 **`/config/lnh.db`**（SQLite）。环境变量仅在**首次启动**作为一次性初始化种子，之后以数据库为准。

路径说明：容器里看到的是挂载后的路径，例如
- 宿主 `/mnt/sata3-1/mp3` → 容器 `/music`，扫描填 `/music`（compose 默认已挂载此路径）

注意事项：
- 音乐卷**必须可读写**（应用会把标签/封面/歌词写回文件），compose 里默认就是读写，别改成 `:ro`。
- 配置卷 `lnh_config:/config` 保存数据库（密码/Key/选项）与扫描进度，重启不丢失。
- 可选启用 AcoustID 指纹识别：在「设置」页填 API Key（也可用 compose 环境变量做首次种子）。
- 关闭：`docker compose down`；查看日志：`docker compose logs -f`。

## 项目结构

```
LNH-musictag/
  Dockerfile              # 多阶段：node 构建前端 → Go 静态编译 → alpine + ffmpeg（格式转换）
  docker-compose.yml      # 端口 10248、音乐卷、配置卷
  .dockerignore
  main.go                 # HTTP 服务、路由、内存存储、异步扫描、多源 handler、embed dist
  auth.go                 # 登录鉴权、随机初始密码（数据库）、改密
  helpers.go              # base64 / 远程抓取 / 循环去后缀
  scrapejob.go            # 批量刮削后台任务（进度 + 元数据补全）
  internal/
    model/                # Track / DuplicateGroup
    store/config.go       # SQLite 配置存储（账号/密码/Key/选项）
    audiofmt/             # 文件头魔数检测真实音频编码格式（文件重命名用）
    taglibx/              # 标签读写封装（go-taglib，含 LYRICS 歌词）
    dedup/                # SHA256 去重 + Fingerprinter 接口
    scrape/               # 11 个源：sources/musicbrainz/itunes/netease/qq/kugou/kuwo/migu/bilibili/qishui/bodian/qianqian + lyrics
  web/                    # Vue3 + Vite 前端（构建产物 dist 被 go:embed 编译进二进制）
    src/
      main.js             # Vue 入口
      App.vue             # 整合布局：左侧树形音乐列表 + 右侧功能区 tab
      store.js            # 响应式全局状态 + API 封装
      api.js              # fetch 封装（401 自动跳登录）
      views/Login.vue     # 登录页
      views/Settings.vue  # 设置（改密 + 数据库配置）
      components/         # MusicTree/TreeNode（树形列表）、EditPanel、BatchPanel、DedupPanel、Toast
```

## 刮削源

| 源 | 类型 | 封面 | 歌词 |
|---|---|---|---|
| 自动 | 依次尝试 | 由命中的源决定 | 自动选择 |
| MusicBrainz | 国际 | Cover Art Archive ✓ | lrclib |
| iTunes | 国际 | artworkUrl ✓ | lrclib |
| 网易云音乐 | 国内 | 专辑详情 ✓ | 网易云 LRC ✓ |
| QQ音乐 | 国内 | albummid ✓ | QQ 歌词 |
| 酷狗音乐 | 国内 | 专辑封面 ✓ | 自动 |
| 酷我音乐 | 国内 | - | 自动 |
| 咪咕音乐 | 国内 | picList ✓ | 自动 |
| 哔哩哔哩 | 国内 | - | 自动 |
| 汽水音乐 | 国内（Douyin，可能被反爬拦截） | - | 自动 |
| 波点音乐 | 国内（无公开搜索接口，占位） | - | 自动 |
| 千千音乐 | 国内（旧接口，可能失效） | - | 自动 |

> 汽水/波点/千千部分网络/机房可能被反爬拦截或接口失效；`auto` 会自动跳过失败源，依次尝试可用源。歌词优先用命中的国内源 ID 取 LRC，否则按标题+艺术家搜网易云，再退 lrclib。

## API 一览

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/login` | `{user,pass}` 登录，下发会话 cookie |
| POST | `/api/logout` | 退出登录 |
| GET | `/api/me` | 登录状态 |
| POST | `/api/change-password` | `{oldPass,newPass}` 改密码（存数据库） |
| POST | `/api/scan` | `{dir}` 开始异步扫描，立即返回 |
| GET | `/api/scan/status` | 扫描进度 `{running,added,total,done}` |
| GET | `/api/tracks` | 曲目列表（扫描中可反复拉取增量渲染） |
| GET | `/api/tracks/{id}/cover` | 读内嵌封面 |
| POST | `/api/tracks/{id}/tags` | `{tags,clear}` 写标签（含 `LYRICS`） |
| POST | `/api/tracks/{id}/cover` | `{url\|dataBase64\|clear}` 写封面 |
| POST | `/api/tracks/fix-title` | `{ids}` 批量去音频后缀标题（默认全部） |
| POST | `/api/rename-files` | `{ids}` 重命名文件：去重复/错误音频后缀（校验真实编码），`发如雪.mp3.flac→发如雪.flac` |
| GET | `/api/duplicates` | 重复分组（hash + fingerprint） |
| GET | `/api/settings` | 读取数据库配置（不含密码） |
| POST | `/api/settings` | 保存配置（账号/Key/选项/目录） |
| GET | `/api/scrape/sources` | 可用刮削源列表 |
| GET | `/api/scrape/search?q=&source=` | 按源搜索（source 可省略=auto） |
| POST | `/api/tracks/{id}/scrape` | 应用刮削（写标签+封面+可选歌词，含补全元数据） |
| GET | `/api/lyrics?source=&title=&artists=` | 获取歌词文本 |
| POST | `/api/scrape/batch` | `{ids,fetchCover,fetchLyrics,source}` 批量刮削，返回 jobId |
| GET | `/api/scrape/jobs/{id}` | 批量任务进度 |

## 已知限制

- 服务监听 `:10248`，经 compose 映射到宿主 `10248`，局域网内可访问。
- **刮削源各有速率限制**（MusicBrainz ~1 请求/秒）；批量刮削逐曲间已内置 1.1s 节流。
- 汽水/波点/千千依赖第三方接口，可能被反爬拦截或失效，`auto` 会自动跳过。
- AcoustID 指纹识别需在「设置」页填 API Key；默认走文本搜索，无需 key。
- 封面写入对 WAV 等容器依赖 TagLib 支持；主流 MP3/FLAC/M4A 均支持。
