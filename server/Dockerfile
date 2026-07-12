ARG GO_VERSION=1.26

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS cli-builder

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS="${TARGETOS}" GOARCH="${TARGETARCH}" \
    go build -trimpath -ldflags="-s -w" -o /out/nethernode ./cmd/nethernode

FROM maven:3.9-eclipse-temurin-25 AS paper-plugin-builder

WORKDIR /src

COPY server/plugins/nethernode-admin/ ./

RUN mvn --batch-mode --no-transfer-progress package

FROM itzg/minecraft-server:stable-java25

LABEL org.opencontainers.image.title="NetherNode Minecraft Runtime"
LABEL org.opencontainers.image.description="Lean Minecraft Paper crossplay runtime wrapper for NetherNode."

COPY --from=cli-builder /out/nethernode /usr/local/bin/nethernode
COPY --from=paper-plugin-builder /src/target/NetherNodeAdmin.jar /plugins/NetherNodeAdmin.jar
