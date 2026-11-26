FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY . .

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" \
    -o /out/linkforged ./cmd/linkforged

FROM scratch

COPY --from=build /out/linkforged /linkforged
USER 65532:65532
EXPOSE 7070
ENTRYPOINT ["/linkforged"]
