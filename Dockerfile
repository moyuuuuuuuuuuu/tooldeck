FROM node:22-bookworm-slim AS web
WORKDIR /src
COPY web/package*.json ./
RUN npm ci --ignore-scripts
COPY web/ ./
RUN npm run build

FROM golang:1.25-bookworm AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -trimpath -o /tooldeck ./cmd/server

FROM docker:28-cli
RUN apk add --no-cache ca-certificates
COPY --from=go /tooldeck /usr/local/bin/tooldeck
COPY --from=web /src/dist /web
COPY runtimes/ /runtime-context/runtimes/
ENV TOOLDECK_DATA_DIR=/data TOOLDECK_WEB_DIR=/web
EXPOSE 8080
ENTRYPOINT ["tooldeck"]
