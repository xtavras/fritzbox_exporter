FROM golang:alpine AS build
WORKDIR /app
COPY go.mod ./
COPY go.sum ./
RUN go mod download
COPY . .
RUN go build -o /fritzbox_exporter

FROM alpine:latest
WORKDIR /
COPY --from=build /fritzbox_exporter /fritzbox_exporter
COPY metrics-lua.json ./
COPY metrics-upnp.json ./
EXPOSE 9042

ENTRYPOINT [ "sh", "-c", "/fritzbox_exporter -username ${USERNAME} -password ${PASSWORD} -metrics-upnp metrics-upnp.json -metrics-lua metrics-lua.json" ]