FROM golang:1.26-alpine AS build
RUN apk add --no-cache git
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY daemon/ daemon/
COPY cli/ cli/
COPY shared/ shared/

ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags='-s -w' -o /out/lightscaled ./daemon \
 && go build -trimpath -ldflags='-s -w' -o /out/lightscale  ./cli

FROM alpine:3.20
RUN apk add --no-cache ca-certificates

RUN addgroup -S -g 1500 lightscale
COPY --from=build /out/lightscaled /usr/local/bin/lightscaled
COPY --from=build /out/lightscale  /usr/local/bin/lightscale

EXPOSE 51820/udp

CMD ["/usr/local/bin/lightscaled", "--config", "/etc/lightscale/lightscale.toml"]
