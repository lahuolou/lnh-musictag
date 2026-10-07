# LNH-MusicTag

纯 **Docker 部署**的自托管 Web 音乐标签工具（形态类似 Music Tag Web）：**标签读写 + 歌曲去重 + 多源元数据/歌词刮削 + 批量修正**，全部操作通过浏览器后台完成，无本地 CLI。

技术栈：**go-taglib（go.senan.xyz/taglib，WASM 封装 TagLib v2.1，无 CGo）+ 纯 Go SHA256 去重 + 多刮削源（MusicBrainz / iTunes / 网易云 / QQ音乐）+ 歌词（网易云 / lrclib）**。

## 核心能力

| 能力 | 实现 | 说明 |
|---|---|---|
| ① 标签读写 | `go.senan.xyz/taglib`（WASM TagLib，无 CGo，MP3/FLAC/M4A/OGG/WAV/WMA…），多值标签、内嵌封面 | 写标签/封面/歌词 |
| ② 歌曲去重 | SHA256 文件哈希（默认）+ 音频指纹（可插拔） | ④ 重复检测 |
| ③ 多源刮削 | MusicBrainz（国际）/ iTunes（国际）/ 网易云 / QQ音乐（国内） | 搜索/应用刮削可选源，含封面 |
| ④ 歌词 | 网易云 LRC（国内）+ lrclib（国际），自动选择 | 单曲获取/编辑/保存，批量「含歌词」 |
| ⑤ 批量 | 勾选曲目 → 批量刮削（含封面/含歌词/选源）+ 进度条 | ⑤ 批量刮削 |
| ⑥ 批量修正标题 | 去音频后缀（`广岛之恋.mp3 → 广岛之恋`） | 「修正标题(去后缀)」 |

## 快速开始（Docker）

前置：目标机已装 Docker（含 compose）。镜像为多阶段构建的静态二进制（`scratch` 运行层，约 10MB，带 CA 证书，构建时按目标机架构自动适配）。

```bash
# 1) 在项目目录构建并启动
cd /path/to/LNH-musictag
docker compose up -d --build

# 2) 先编辑 docker-compose.yml，把 volumes 里的 D:/Music 改成你的真实音乐目录
```

浏览器访问 **http://<主机IP>:10248**：

0. **后台登录**：账号默认 `admin`（compose 的 `LNH_ADMIN_USER`）。密码：
   - `LNH_ADMIN_PASS` **留空** → 首次启动自动生成**随机初始密码**，打印在容器日志（`docker compose logs`），并持久化到配置卷 `/config`（重启不丢失）；登录后点右上角"修改密码"换成自己的。
   - 或直接在 compose 里填 `LNH_ADMIN_PASS=你的密码` 固定。
1. 输入音频目录的**容器内路径**（如 `/music`）→ 扫描
2. 点左侧曲目 → 右侧编辑标签；**刮削搜索可选来源**（自动/MusicBrainz/iTunes/网易云/QQ音乐），结果含源徽章+封面，「应用(含封面)」「仅标签」
3. 右侧下方**歌词 LYRICS**：点「获取歌词」自动抓取（网易云/lrclib）填入，可编辑，「保存标签」写入；「清除歌词」清空
4. 列表勾选曲目 → ⑤ 批量刮削（选源、勾「含封面」「含歌词」）→ 进度条 + 结果汇总；「修正标题(去后缀)」批量去音频后缀
5. ④ 重复检测 列出 SHA256 相同的文件组

路径说明：容器里看到的是挂载后的路径，例如
- 宿主 `/mnt/sata3-1/mp3` → 容器 `/music`，扫描填 `/music`（compose 默认已挂载此路径）
- 宿主 `/mnt/sata3-1/mp3/pop` → 容器 `/music/pop`，扫描填 `/music/pop`

注意事项：
- **后台账号密码与音乐路径都已写在 `docker-compose.yml`**，部署时按需修改 `LNH_ADMIN_PASS` 和 `volumes` 即可。
- 音乐卷**必须可读写**（应用会把标签/封面/歌词写回文件），compose 里默认就是读写，别改成 `:ro`。
- 可选启用 AcoustID：在 compose 的 `environment` 里填 `ACOUSTID_API_KEY`。
- 关闭：`docker compose down`；查看日志：`docker compose logs -f`。

## 项目结构

```
LNH-musictag/
  Dockerfile              # 多阶段静态构建（golang:1.27-alpine → scratch）
  docker-compose.yml      # 端口 10248、音乐卷、环境变量
  .dockerignore
  main.go                 # HTTP 服务、路由、内存存储、多源 handler
  auth.go                 # 登录鉴权、随机初始密码、改密持久化
  helpers.go              # base64 / 远程抓取 / 标题去后缀
  scrapejob.go            # 批量刮削后台任务（进度）
  internal/
    model/                # Track / DuplicateGroup
    taglibx/              # 标签读写封装（go-taglib，含 LYRICS 歌词）
    dedup/                # SHA256 去重 + Fingerprinter 接口
    scrape/               # 多源：sources/itunes/netease/qq/musicbrainz + lyrics
  web/index.html          # 单页前端（go:embed 内嵌）
```

## 刮削源

| 源 | 类型 | 元数据 | 封面 | 歌词 |
|---|---|---|---|---|
| 自动 | - | 依次尝试 | 由命中的源决定 | 自动选择 |
| MusicBrainz | 国际 | ✓ | Cover Art Archive ✓ | lrclib |
| iTunes | 国际 | ✓ | artworkUrl ✓ | lrclib |
| 网易云音乐 | 国内 | ✓ | 专辑详情兜底 ✓ | 网易云 LRC ✓ |
| QQ音乐 | 国内 | ✓ | albummid ✓ | QQ 歌词 |

歌词自动策略：优先用命中的网易云/QQ 歌曲 ID 取歌词；否则按标题+艺术家搜网易云，再退 lrclib（国际）。

## API 一览

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/login` | `{user,pass}` 后台登录，下发会话 cookie |
| POST | `/api/logout` | 退出登录 |
| GET | `/api/me` | 当前登录状态（200 已登录 / 401 未登录） |
| POST | `/api/change-password` | `{oldPass,newPass}` 修改后台密码 |
| POST | `/api/scan` | `{dir}` 扫描目录 |
| GET | `/api/tracks` | 曲目列表 |
| GET | `/api/tracks/{id}/cover` | 读内嵌封面 |
| POST | `/api/tracks/{id}/tags` | `{tags,clear}` 写标签（含 `LYRICS`） |
| POST | `/api/tracks/{id}/cover` | `{url\|dataBase64\|clear}` 写封面 |
| POST | `/api/tracks/fix-title` | `{ids}` 批量去音频后缀 |
| GET | `/api/duplicates` | 重复分组（hash + fingerprint） |
| GET | `/api/scrape/sources` | 可用刮削源列表 |
| GET | `/api/scrape/search?q=&source=` | 按源搜索（source 可省略=auto） |
| POST | `/api/tracks/{id}/scrape` | 应用刮削（写标签+封面+可选歌词） |
| GET | `/api/lyrics?source=&title=&artists=` | 获取歌词文本 |
| POST | `/api/scrape/batch` | `{ids,fetchCover,fetchLyrics,source}` 批量刮削，返回 jobId |
| GET | `/api/scrape/jobs/{id}` | 批量任务进度（done/total/current/results） |

## 关于"指纹去重"（策略二）的现状与启用

方案里的 **musiclab**（`github.com/drgolem/musiclab`，Chromaprint 指纹 + AcoustID 查询）已核实，但其 README 明确写着 **"Requires CGo for FLAC (libflac) and MP3 (mpg123) decoders"**，且为 `v0.0.0-...` 未发版模块，直接依赖会让 Docker 镜像编译链脆弱。

因此指纹做了**可插拔**设计，不阻塞主链路：

- `internal/dedup.Fingerprinter` 接口：`Fingerprint(path) (string, error)`
- 默认实现 `FpcalcFingerprinter`：容器内存在 Chromaprint 的 `fpcalc` 命令则自动启用并做指纹去重；否则回退 `NoopFingerprinter`（仅 SHA256）。
- 前端 ④ 面板顶部的"指纹：已启用/未启用"徽标即反映该状态。

启用方式（二选一）：
1. 在镜像中安装 Chromaprint（在 Dockerfile 运行层加入 `fpcalc`），最简单。
2. 接入 musiclab 库（需在镜像编译阶段解决 libflac/mpg123 依赖），建议独立分支做。

## 已知限制

- 服务监听 `:10248`（容器内全部接口），经 compose 映射到宿主 `10248`，局域网内可访问。
- **刮削源各有速率限制**（MusicBrainz ~1 请求/秒）；批量刮削逐曲间已内置 1.1s 节流，几十上百首会较慢。
- **QQ 音乐**在国内网络通常可用；某些网络/机房可能被 QQ 拦截（旧接口 502、新接口空结果），此时自动会退到网易云/lrclib。网易云接口部分场景需带浏览器 UA/Referer，已内置处理。
- **歌词**：网易云对中文覆盖最好；国际歌曲走 lrclib。若命中的源无对应歌词，会返回"未找到歌词"，不影响标签/封面写入。
- AcoustID 录音识别需注册 API key（`ACOUSTID_API_KEY`）；当前默认走文本搜索，无需 key。
- 封面写入对 WAV 等容器依赖 TagLib 支持；主流 MP3/FLAC/M4A 均支持。
