FROM docker.io/library/golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/webtrees-mcp .

FROM docker.io/library/alpine:3.22

RUN addgroup -S webtrees && adduser -S -G webtrees webtrees

COPY --from=build /out/webtrees-mcp /usr/local/bin/webtrees-mcp

USER webtrees
ENTRYPOINT ["/usr/local/bin/webtrees-mcp"]
