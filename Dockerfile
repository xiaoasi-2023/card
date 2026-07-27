ARG NODE_BASE_IMAGE=public.ecr.aws/docker/library/node:24-bookworm-slim
ARG GO_BASE_IMAGE=public.ecr.aws/docker/library/golang:1.24-bookworm
ARG RUNTIME_BASE_IMAGE=public.ecr.aws/docker/library/debian:bookworm-slim

FROM ${NODE_BASE_IMAGE} AS frontend-builder

WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install --no-audit --no-fund; fi
COPY frontend/ ./
RUN npm run build

FROM ${GO_BASE_IMAGE} AS backend-builder

WORKDIR /src/backend
COPY backend/go.mod backend/go.sum* ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/card .

FROM ${RUNTIME_BASE_IMAGE}

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=backend-builder /out/card /app/card
COPY --from=frontend-builder /src/frontend/dist /app/web

ENV APP_ADDR=:3000 \
    WEB_ROOT=/app/web \
    DATABASE_PATH=/app/data/card.db

EXPOSE 3000
VOLUME ["/app/data"]

ENTRYPOINT ["/app/card"]
