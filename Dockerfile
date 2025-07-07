FROM golang:alpine AS build
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN go build -o /fritzbox_exporter


FROM alpine:latest
RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=build /fritzbox_exporter /app/
COPY metrics-upnp.json metrics-lua.json /app/

EXPOSE 9042

ENTRYPOINT [ "sh", "-c", "/app/fritzbox_exporter -username ${USERNAME} -password ${PASSWORD} -gateway-lua-url ${GATEWAY_URL_LUA} -gateway-upnp-url ${GATEWAY_URL_UPNP} -listen-address ${LISTEN_ADDRESS} -metrics-upnp /app/metrics-upnp.json -metrics-lua /app/metrics-lua.json" ]
