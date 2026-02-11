# syntax=docker/dockerfile:1

########## build ##########
FROM golang:1.25-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates

# Кешируем зависимости
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/app .

########## runtime ##########
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app

COPY --from=build /out/app /app/app
COPY --chown=nonroot:nonroot templates /app/templates
COPY --chown=nonroot:nonroot static /app/static
COPY --chown=nonroot:nonroot config.json /app/config.json


EXPOSE 8080
ENV PORT=8080

# IMPORTANT: set 0.0.0.0 in config.json bc container will listen only loopback in 127.0.0.1 case
ENTRYPOINT ["/app/app"]

