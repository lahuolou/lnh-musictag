# ---- 构建阶段 ----
# go-taglib 是 WASM 封装，无 CGo，可产出纯静态二进制
FROM golang:1.27-alpine AS build
WORKDIR /src

# 先复制依赖清单以利用缓存
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 保证静态链接；-s -w 精简符号表
RUN apk add --no-cache ca-certificates \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/lnh-musictag .

# ---- 运行阶段：scratch 最小镜像 ----
FROM scratch
# 复制 CA 证书，供应用 HTTPS 访问 MusicBrainz / Cover Art Archive
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/lnh-musictag /lnh-musictag

EXPOSE 10248
# 音乐目录挂载点（宿主目录映射到此，需读写，应用会把标签/封面写回文件）
VOLUME ["/music"]
ENTRYPOINT ["/lnh-musictag"]
