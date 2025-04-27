## syntax=docker/dockerfile:1
FROM golang:1.24
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /sonarr-anime-importer

ARG PORT=3333
EXPOSE ${PORT}

CMD ["/sonarr-anime-importer"]
