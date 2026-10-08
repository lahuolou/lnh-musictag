# LNH-MusicTag

纯 **Docker 部署**的自托管 Web 音乐标签管理工具：**标签读写 + 歌曲去重 + 多源元数据/歌词刮削 + 批量修正 + 格式转换 + 异步扫描**，全部操作通过浏览器后台完成，无本地 CLI。

技术栈：**Go 后端（go-taglib WASM 封装 TagLib，无 CGo）+ 纯 Go SHA256 去重 + 10 个刮削源（国内优先）+ 歌词 + Vue3 多页面前端**，配置存 **SQLite**（不依赖环境变量）。

前端用 **Vue 3 + Vite**（比 React 运行时更轻），构建产物约 90KB JS，`go:embed` 进单个静态二进制，Docker 多阶段构建（node → Go → scratch），运行时无 node 依赖、不额外占内存。

## 核心能力

| 能力 | 实现 | 说明 |
|---|---|---|
| ① 标签读写 | `go.senan.xyz/taglib`（WASM，无 CGo，MP3/FLAC/M4A/OGG/WAV/WMA…），多值标签、内嵌封面 | 写标题/艺术家/专辑/专辑艺术家/流派/年份/曲目号/封面/歌词 |
| ② 异步扫描 | `/api/scan` 后台任务，**边扫边入库**；前端轮询 `/api/scan/status` 增量渲染曲目，不等全部扫完 | 扫描期间即见曲目 |
| ③ 歌曲去重 | SHA256 文件哈希（**打开去重页时按需计算并缓存**）+ **同名多格式**（同名不同格式保留音质最佳）+ **艺术家+标题**（同歌不同名） | 去重页 |
| ④ 多源刮削 | **10 个源**：网易云 / QQ / 酷狗 / 酷我 / 咪咕 / 哔哩哔哩 / 汽水 / 波点（国内）＋ MusicBrainz / iTunes（国际），**国内源优先** | 搜索/应用，自动依次尝试 |
| ⑤ 补全元数据 | MusicBrainz release 详情回填**专辑艺术家、流派、年代、曲目号**；写入标签 | 批量/单曲刮削均可 |
| ⑥ 歌词 | 网易云 LRC + lrclib，自动选择 | 单曲获取/编辑/保存，批量「含歌词」 |
| ⑦ 批量 | 勾选曲目 → 批量刮削；**字段可选**（海报/歌名/艺术家/流派/年代/专辑艺术家/曲目号/歌词），**智能跳过**已有标签的曲目；**后台运行**：切页进度仍可见（顶栏进度徽标），每完成一首**列表实时增量更新**（不整表刷新、不打断勾选/滚动） | 批量页 |
| ⑧ 去后缀默认执行 | ① 文件重命名：扫描时按**真实编码格式**（读文件头魔数）去重复/错误音频后缀，`发如雪.mp3.flac → 发如雪.flac`、`发如雪.mp3.mp3 → 发如雪.mp3`；② 标签标题去后缀 `广岛之恋.mp3 → 广岛之恋` | 均默认开启，可在设置关闭 |
| ⑨ 配置存数据库 | 账号/密码/API Key/选项存 `SQLite`（`/config/lnh.db`），不再用环境变量 | 设置页读写 |
| ⑩ 列表搜索 | 左侧音乐列表实时搜索：标题/艺术家/专辑/文件名 | 本地过滤 |
| ⑪ 主题切换 | 黑/白天模式，现代扁平化风格，随偏好记忆 | 顶栏 🌙/☀️ |
| ⑫ 语言优选源 | 自动源按语言排序：中文→QQ/网易云等国内源优先，外文→iTunes/MusicBrainz 等国际源优先 | auto 源 |
| ⑬ 整合布局 | 单页合并：**左侧表格音乐列表**（列：封面/标题/艺术家/专辑/专辑艺术家/歌词/LRC/年份/风格/大小，可搜索/勾选批量，点击行进编辑）+ **右侧功能区**（编辑｜批量｜去重 子 tab）；设置独立页 | 一体化界面 |
| ⑭ 格式转换 | 弹窗式批量转换：输入格式筛选（FLAC/MP3/APE/WAV…16 种，留空=全部）、目标格式（维持原格式/10 种）、比特率、采样率；保存在同目录、不删源文件 | 批量面板 · ffmpeg |
| ⑮ 列表筛选 | 搜索框 **输入即过滤**（无搜索按钮）+ 快捷筛选：**乱码 / 繁体 / 无歌词 / 无封面 / 无艺术家** | 列表页 |
| ⑯ 乱码修复 | 自动还原 GBK/UTF-8 错读的中文标签（标题/艺术家/专辑等） | 批量面板 |
| ⑰ 简繁体转换 | 标签文本 简体⇄繁体（OpenCC 引擎） | 批量面板 |
| ⑱ 智能扫描 | 已知文件**智能跳过**；扫描**只读标签不哈希**（SHA256 按需在去重页计算），全量扫描速度接近同类工具；扫描中可**暂停/继续**；扫描进度显示已加载/发现/跳过数 | 列表页 |
| ⑲ 保存即重命名 | 编辑页默认从源文件名载入标题/艺术家（`陈小春 - 街角的晚风.flac` → 艺术家=陈小春、标题=街角的晚风）；保存标签时若标题/艺术家与文件名不符，自动把文件重命名为 **`艺术家 - 标题.后缀`** | 编辑页 |
| ⑳ 合唱整理 | 艺术家字段按 `/ ; 、 ， _` 拆分后 **超过 3 位 → 一键改为“合唱”** | 批量面板 |
| ㉑ 去重增强 | 去重新增 **艺术家+标题** 维度分组；每个分组勾选多选、**手动删除**（默认保留最佳音质，绝不自动删） | 去重页 |
| ㉒ 中文习惯优化 | **扫描中按扫描顺序稳定排列**（先扫到在前、递增在后，不乱序）；暂停/完成后点标题、艺术家等表头按 **A-Z 0-9**（中文按拼音）排序；大曲库**分页渲染**（滚动加载不卡）、封面**懒加载**、搜索自动忽略**全角/大小写**、**外挂 .lrc** 识别与一键导入、扫描自动跳过隐藏目录与 NAS 系统目录（@eaDir/#recycle/@__thumb）、去重默认不勾选并显示文件大小/时长、登录防重复提交 | 全局 |

## 快速开始（Docker）

前置：目标机已装 Docker（含 compose）。

> **镜像地址（GHCR）**：`ghcr.io/lahuolou/lnh-musictag:latest`
> 由 GitHub 仓库 `lahuolou/lnh-musictag` 的 `main` 分支自动构建推送（`.github/workflows/docker.yml`），镜像为多阶段构建的静态二进制（scratch 运行层，约 10MB，带 CA 证书，构建时按目标机架构自动适配）。

### 方式一：docker compose（推荐）

```bash
# 1) 拉取镜像并后台启动
docker pull ghcr.io/lahuolou/lnh-musictag:latest
docker compose up -d

# 2) 首次使用请编辑 docker-compose.yml：
#    - 把 /mnt/sata3-1/mp3 改成你的真实音乐目录（默认已挂载为 /music）
#    - 端口默认 10248（host 端口可改）
#    - 账号密码、AcoustID Key 可写在 LNH_ADMIN_USER / LNH_ADMIN_PASS / ACOUSTID_API_KEY（也可不写，首次启动自动生成随机密码，见下）
```

### 方式二：docker run（单条命令）

```bash
docker run -d --name lnh-musictag --restart unless-stopped \
  -p 10248:10248 \
  -v /mnt/sata3-1/mp3:/music \
  -v lnh_config:/config \
  -e LNH_ADMIN_USER=admin \
  -e LNH_ADMIN_PASS='你的初始密码' \
  ghcr.io/lahuolou/lnh-musictag:latest
```

> 不填 `LNH_ADMIN_PASS` 时，首次启动会在容器日志里打印随机初始密码（`docker logs lnh-musictag`），登录后到「设置」改密码（存数据库）。

### 更新到最新版

```bash
docker compose pull && docker compose up -d                          # compose 方式
docker pull ghcr.io/lahuolou/lnh-musictag:latest && docker restart lnh-musictag   # run 方式
```



浏览器访问 **http://<主机IP>:10248**：

1. **登录页**：账号默认 `admin`。**首次启动自动生成随机初始密码**，打印在容器日志（`docker compose logs`），登录后到「设置」改密码（存数据库）。
2. **音乐列表**：输入容器内音频目录（如 `/music`）→ 点「扫描」→ **边扫边出**，扫描中按扫描顺序稳定排列（先扫到在前）；点击行进「编辑」，可勾选多首到「批量」。
3. **功能区**：右侧 **编辑 / 批量 / 去重** 三个子 tab——编辑标签/刮削/歌词；批量补全/乱码修复/简繁转换/格式转换；去重检测（勾选手动删除）。**设置** 独立页，改密与配置。

> 账号、密码、AcoustID Key、默认目录、是否自动去后缀等均保存在 **`/config/lnh.db`**（SQLite）。环境变量仅在**首次启动**作为一次性初始化种子，之后以数据库为准。

路径说明：容器里看到的是挂载后的路径，例如
- 宿主 `/mnt/sata3-1/mp3` → 容器 `/music`，扫描填 `/music`（compose 默认已挂载此路径）

注意事项：
- 音乐卷**必须可读写**（应用会把标签/封面/歌词写回文件），compose 里默认就是读写，别改成 `:ro`。
- 配置卷 `lnh_config:/config` 保存数据库（密码/Key/选项）与扫描进度，重启不丢失。
- 可选填 MusicBrainz/AcoustID API Key（「设置」页，或用 compose 环境变量首次种子），提升国际源匹配；不填也能用文本搜索。
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
  fixencoding.go          # 乱码修复（多轮编码还原 + 文件名回退）
  zhscript.go             # 简繁转换 + 繁体检测（OpenCC）
  scrapejob.go            # 批量刮削后台任务（进度 + 增量推送 + 元数据补全）
  internal/
    model/                # Track / DuplicateGroup
    store/config.go       # SQLite 配置存储（账号/密码/Key/选项）
    audiofmt/             # 文件头魔数检测真实音频编码格式（文件重命名用）
    taglibx/              # 标签读写封装（go-taglib，含 LYRICS 歌词）
    dedup/                # SHA256 / 同名多格式 / 艺术家+标题 去重分组
    scrape/               # 10 个源：sources/netease/qq/kugou/kuwo/migu/bilibili/qishui/bodian/musicbrainz/itunes + lyrics
  web/                    # Vue3 + Vite 前端（构建产物 dist 被 go:embed 编译进二进制）
    src/
      main.js             # Vue 入口
      App.vue             # 整合布局：左侧表格音乐列表 + 右侧功能区 tab
      store.js            # 响应式全局状态 + API 封装
      api.js              # fetch 封装（401 自动跳登录）
      views/Login.vue     # 登录页
      views/Settings.vue  # 设置（改密 + 数据库配置）
      components/         # MusicTree（表格列表）、EditPanel、BatchPanel、DedupPanel、FormatDialog、Toast
```

## 刮削源

| 源 | 类型 | 封面 | 歌词 |
|---|---|---|---|
| 自动 | 依次尝试 | 由命中的源决定 | 自动选择 |
| 网易云音乐 | 国内 | 专辑详情 ✓ | 网易云 LRC ✓ |
| QQ音乐 | 国内 | albummid ✓ | QQ 歌词 |
| 酷狗音乐 | 国内 | 专辑封面 ✓ | 自动 |
| 酷我音乐 | 国内 | - | 自动 |
| 咪咕音乐 | 国内 | picList ✓ | 自动 |
| 哔哩哔哩 | 国内 | - | 自动 |
| 汽水音乐 | 国内（Douyin，可能被反爬拦截） | - | 自动 |
| 波点音乐 | 国内（无公开搜索接口，占位） | - | 自动 |
| MusicBrainz | 国际 | Cover Art Archive ✓ | lrclib |
| iTunes | 国际 | artworkUrl ✓ | lrclib |

> 中文查询 `auto` 自动按 国内源 → 国际源 依次尝试，外文查询反之；失败源自动跳过。歌词优先用命中的国内源 ID 取 LRC，否则按标题+艺术家搜网易云，再退 lrclib。**千千音乐（百度）旧接口已下线（HTTPS 证书不匹配且返回 500），已从源列表移除。**

## API 一览

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/login` | `{user,pass}` 登录，下发会话 cookie |
| POST | `/api/logout` | 退出登录 |
| GET | `/api/me` | 登录状态 |
| POST | `/api/change-password` | `{oldPass,newPass}` 改密码（存数据库） |
| POST | `/api/scan` | `{dir}` 开始异步扫描，立即返回 |
| GET | `/api/scan/status` | 扫描进度 `{running,paused,added,total,done,skipped}` |
| POST | `/api/scan/pause` | `{paused}` 暂停/继续扫描 |
| GET | `/api/tracks` | 曲目列表（按扫描顺序稳定返回，扫描中可反复拉取增量渲染） |
| GET | `/api/tracks/{id}/cover` | 读内嵌封面 |
| GET | `/api/tracks/{id}/lrcfile` | 读同目录外挂 `.lrc` 歌词 |
| POST | `/api/tracks/{id}/tags` | `{tags,clear}` 写标签（含 `LYRICS`）；标题/艺术家与文件名不符时自动重命名为 `艺术家 - 标题.后缀` |
| POST | `/api/tracks/{id}/cover` | `{url\|dataBase64\|clear}` 写封面 |
| GET | `/api/duplicates` | 重复分组（hash + format + tags）；缺失的 SHA256 在此按需计算并缓存 |
| POST | `/api/duplicates/remove` | `{ids}` 手动删除选中的重复文件 |
| POST | `/api/set-chorus` | `{ids}` 艺术家>3 位 → 改“合唱” |
| POST | `/api/fix-encoding` | `{ids}` 乱码标签修复（GBK/UTF-8） |
| POST | `/api/convert-script` | `{ids,to}` 标签简繁转换 |
| POST | `/api/convert` | `{ids,target,inputFormats,bitrate,sampleRate}` ffmpeg 格式转换 |
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
- 汽水/波点依赖第三方接口，可能被反爬拦截或失效，`auto` 会自动跳过。
- 封面写入对 WAV 等容器依赖 TagLib 支持；主流 MP3/FLAC/M4A 均支持。
