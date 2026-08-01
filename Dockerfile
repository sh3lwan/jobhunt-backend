# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.23-bookworm AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/jobhunter ./cmd

# ---- runtime stage ----
FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/jobhunter /app/jobhunter
# main.go calls godotenv.Load(), which errors if no .env file is present.
# Ship an empty one so the process starts in a fresh clone (no .env in git);
# all real config comes from compose `environment:` / env_file values.
RUN touch /app/.env

# CV uploads are written here at runtime; compose mounts a volume over it.
RUN mkdir -p /app/storage/uploads

EXPOSE 8080
CMD ["/app/jobhunter"]
