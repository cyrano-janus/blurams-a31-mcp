FROM golang:1.27.2 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /blurams-mcp ./cmd/blurams-mcp

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /blurams-mcp /blurams-mcp
EXPOSE 8000
ENTRYPOINT ["/blurams-mcp"]
