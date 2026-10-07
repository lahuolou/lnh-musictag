# LNH-MusicTag

纯 **Docker 部署**的自托管 Web 音乐标签工具（形态类似 Music Tag Web）：**标签读写 + 歌曲去重 + 元数据刮削**，全部操作通过浏览器后台完成，无本地 CLI。

技术栈：**go-taglib（go.senan.xyz/taglib，WASM 封装 TagLib v2.1，无 CGo）+ 纯 Go SHA256 去重 + MusicBrainz 刮削**。

## 三大能力（均已端到端验证）

| 能力 | 实现 | 验证结果 |
|---|---|---|
| ① 标签读写 | `go.senan.xyz/taglib`（WASM TagLib，无 CGo，多格式 MP3/FLAC/M4A/OGG/WAV/WMA…），支持多值标签、内嵌封面 | 写入→回读→磁盘校验 ✓ |
| ② 歌曲去重 | 策略一：SHA256 文件哈希（默认激活，纯 Go）；策略二：音频指纹（可插拔，见下） | 相同副本自动分组 ✓ |
| ③ 元数据刮削 | MusicBrainz 搜索 + Cover Art Archive 封面，无需 API key | 搜索→写标签→抓封面→回读 ✓ |

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
2. 点左侧曲目 → 右侧编辑标签 / MusicBrainz 搜索刮削
3. ④ 重复检测 列出 SHA256 相同的文件组

路径说明：容器里看到的是挂载后的路径，例如
- 宿主 `/mnt/sata3-1/mp3` → 容器 `/music`，扫描填 `/music`（compose 默认已挂载此路径）
- 宿主 `/mnt/sata3-1/mp3/pop` → 容器 `/music/pop`，扫描填 `/music/pop`

注意事项：
- **后台账号密码与音乐路径都已写在 `docker-compose.yml`**，部署时按需修改 `LNH_ADMIN_PASS` 和 `volumes` 即可。
- 音乐卷**必须可读写**（应用会把标签/封面写回文件），compose 里默认就是读写，别改成 `:ro`。
- 可选启用 AcoustID：在 compose 的 `environment` 里填 `ACOUSTID_API_KEY`。
- 关闭：`docker compose down`；查看日志：`docker compose logs -f`。

## 项目结构

```
LNH-musictag/
  Dockerfile              # 多阶段静态构建（golang:1.27-alpine → scratch）
  docker-compose.yml      # 端口 10248、音乐卷、环境变量
  .dockerignore
  main.go                 # HTTP 服务、路由、内存存储
  auth.go                 # 登录鉴权、随机初始密码、改密持久化
  helpers.go              # base64 / 远程抓取
  internal/
    model/                # Track / DuplicateGroup
    taglibx/              # 标签读写封装（go-taglib）
    dedup/                # SHA256 去重 + Fingerprinter 接口
    scrape/               # MusicBrainz + Cover Art Archive（+ AcoustID 可选）
  web/index.html          # 单页前端（go:embed 内嵌）
```

## API 一览

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/api/login` | `{user,pass}` 后台登录，下发会话 cookie |
| POST | `/api/logout` | 退出登录 |
| GET | `/api/me` | 当前登录状态（200 已登录 / 401 未登录） |
| POST | `/api/change-password` | `{oldPass,newPass}` 修改后台密码（需登录） |
| POST | `/api/scan` | `{dir}` 扫描目录 |
| GET | `/api/tracks` | 曲目列表 |
| GET | `/api/tracks/{id}/cover` | 读内嵌封面 |
| POST | `/api/tracks/{id}/tags` | `{tags,clear}` 写标签 |
| POST | `/api/tracks/{id}/cover` | `{url\|dataBase64\|clear}` 写封面 |
| GET | `/api/duplicates` | 重复分组（hash + fingerprint） |
| GET | `/api/scrape/search?q=` | MusicBrainz 录音搜索 |
| POST | `/api/tracks/{id}/scrape` | 应用刮削（写标签+封面） |

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
- MusicBrainz 有速率限制（约 1 请求/秒），批量刮削需自行加节流。
- AcoustID 录音识别需注册 API key（环境变量 `ACOUSTID_API_KEY`）；当前默认走 MusicBrainz 文本搜索，无需 key。
- 封面写入对 WAV 等容器依赖 TagLib 支持；主流 MP3/FLAC/M4A 均支持。
