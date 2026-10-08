# ---- 前端构建：Vue3 + Vite，产出静态产物 dist ----
FROM node:20-alpine AS fe
WORKDIR /fe
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- Go 构建阶段 ----
# go-taglib 是 WASM 封装，无 CGo，可产出纯静态二进制
FROM golang:1.27-alpine AS build
WORKDIR /src

# 先复制依赖清单以利用缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 把前端静态产物拷入 web/dist，供 go:embed 编译进二进制
COPY --from=fe /fe/dist ./web/dist
# CGO_ENABLED=0 保证静态链接；-s -w 精简符号表
RUN apk add --no-cache ca-certificates \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/lnh-musictag .

# ---- 运行阶段：alpine + ffmpeg（格式转换依赖 ffmpeg）----
FROM alpine:3.20
# ffmpeg 供格式转换；ca-certificates 供应用 HTTPS 访问 MusicBrainz/网易云/QQ 等刮削源
RUN apk add --no-cache ca-certificates ffmpeg
COPY --from=build /out/lnh-musictag /lnh-musictag

EXPOSE 10248
# 音乐目录挂载点（宿主目录映射到此，需读写，应用会把标签/封面/歌词写回文件）
VOLUME ["/music"]
ENTRYPOINT ["/lnh-musictag"]
