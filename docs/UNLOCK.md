# 音乐解锁插件（tool.unlock）

把 Unlock Music（[git.unlock-music.dev/um/web](https://git.unlock-music.dev/um/web)）的解密能力用**纯 Go 重写**并接入 LNH-MusicTag。不引入 WASM runtime、无外部依赖，密钥表全部内嵌，体积与内存开销都很小。

## 支持格式

| 格式 | 来源 | 说明 |
|---|---|---|
| NCM / UC | 网易云 | AES-128-ECB + RC4 变体 keyBox；内嵌标题/歌手/专辑可写入 |
| QMC 全系 | QQ 音乐 | qmc0/1/2/3/4/6/8、qmcflac/qmcogg、mflac/mflac0/mgg/mgg0/mgg1/mggl/mmp4、tkm、bkc*、hex 后缀（微云）、.cache 缓存、tm0/tm3/tm2/tm6 |
| KGM / VPR / KGMA | 酷狗 | 掩码表内嵌（无需外部 kgm.mask 文件） |
| KWM | 酷我 | 8 字节 key 派生 32 字节掩码 |
| XM | 虾米 | wav/mp3/flac/m4a 直通也走此通道 |
| MG3D | 咪咕 | 动态探测 0x20 段密钥 |
| X2M / X3M | 喜马拉雅 | scramble 表内嵌 |
| JOOX（ofl_en） | JOOX | **实验性**：仅头部识别与版本检测。上游 `@unlock-music/joox-crypto` 已从 npm 下架且源码无法获取（私有 registry 有 Cloudflare 保护），密钥派生算法无法移植，会返回明确错误提示 |
| mflach | QQ 音乐 | 与上游一致：需 unlock-music CLI，网页版不支持 |

## 接入方式

1. **执行逻辑（源码扩展点）**：`internal/unlock/` 解密核心 + `unlock_plugin.go` 宿主 API，已编译进主程序。重新构建镜像后生效。
2. **插件声明（后台对接）**：`tool.unlock` 已在 `registerBuiltinPlugins` **内置注册**（与 `scrape.*`、`tool.ffmpeg` 同机制），重启不丢、插件页自动列出并显示"音乐解锁"。
   - `unlock.plugin.json` 保留作导入参考：由于内置同名，后台导入会提示"插件已存在"，属正常现象，无需导入。
3. **API**：
   - `POST /api/unlock`：multipart 上传（字段名 `file`），同步解密，返回 `{token, ext, title, artist, album, songId, download}`。请求体上限 256MiB（挂在内层 limitBody 之外，仍要求登录 + `X-Requested-With: LNH-MusicTag` 头）。
   - `GET /api/unlock/dl/{token}`：下载解密结果。
   - `POST /api/unlock/save`：`{"token":"…","dir":"/music","writeTags":true}`，保存到曲库目录（默认 `scan_dir`），可选写入内嵌元数据，随后自动 `refreshTrack` 登记进曲库。
   - 停用插件（插件页关闭 tool.unlock）后 API 会拒绝并提示先启用。

## 元数据说明

- NCM：解密时解析内嵌 meta（标题/歌手/专辑），`save` 时可选写入。
- QMC：QTag 内嵌的 songId 会随响应返回，可作后续在线补齐元数据的键；当前不做在线拉取（保持小体积）。
- 一般格式：解密后的音频自带标签会被 `taglibx` 读取并登记进曲库，无需额外处理。

## 验证

- 单元测试：`go test ./internal/unlock/`，用 unlock-music 官方 testdata 做黄金数据——mflac_map / mflac_rc4 / mflac0_rc4 / mgg_map / qmc0_static 全部逐字节一致；密钥解密 4 组一致；TencentTea 往返一致。
- 端到端（隔离端口 + 临时库）：登录 → 插件列表 → 上传 .mflac → 解密为 .flac（65536B）→ 下载逐字节比对 → 保存入库 → `/api/tracks` 出现新曲目且标签/时长/采样率正确。

## 备注

- `main.go` 顺带新增 `LNH_PORT` 环境变量支持（默认仍 `:10248`），便于部署与隔离测试。
- 若本次改动需要发布，请按既有规则递增 AppVersion 并创建对应 GitHub Release tag（v1.4.1 起 +0.0.1，v1.4.50 后跳 v1.5.0）。
- 前端暂无解锁入口页面（上传用 curl / API 调用即可）；如需在 Web 界面做"拖拽上传解锁"，需改 `web/src` 并重新构建内嵌 `web/dist`，成本较高，可按需再加。
