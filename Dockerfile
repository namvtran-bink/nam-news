# Quiet, Mac mode in a container: fetch news, render a static page, publish to Cloudflare Pages.
# See docker/README section in README.md for usage.

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/quiet ./cmd/quiet

FROM node:22-bookworm-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates tzdata \
 && rm -rf /var/lib/apt/lists/* \
 && npm install -g wrangler@4 \
 && npm cache clean --force
COPY --from=build /out/quiet /usr/local/bin/quiet
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN mkdir -p /data && chown node:node /data

ENV DB_PATH=/data/quiet.db \
    SITE_DIR=/data/site \
    TZ=Asia/Saigon \
    RUN_AT=05:30
VOLUME /data
USER node
WORKDIR /data
ENTRYPOINT ["entrypoint.sh"]
CMD ["schedule"]
