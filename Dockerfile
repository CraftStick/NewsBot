# syntax=docker/dockerfile:1

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /treesheild-newsbot .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /treesheild-newsbot /app/treesheild-newsbot
ENV TZ=Europe/Moscow
ENTRYPOINT ["/app/treesheild-newsbot"]
