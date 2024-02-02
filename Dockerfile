# https://github.com/jtagcat/dotfiles/blob/main/scripts/template/gobuild.Dockerfile
FROM golang:1.21 AS builder
WORKDIR /wd

COPY go.mod go.sum ./
RUN go mod download

COPY *.go ./
COPY ffmpeg ./ffmpeg
COPY ytdlp ./ytdlp
COPY demucs ./demucs
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o demucs-web

FROM python:3.11-slim
LABEL org.opencontainers.image.source="https://github.com/jtagcat/demucs-web"
WORKDIR /wd

RUN apt-get update && apt-get install -y \
  ffmpeg \
  && rm -rf /var/lib/apt/lists/*
RUN pip install yt-dlp demucs

COPY --from=builder /wd/demucs-web ./
CMD ["./demucs-web"]
EXPOSE 8080

COPY laulupeoassets laulupeoassets
COPY templates templates
