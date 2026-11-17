# Imagem da nuvem do Applyer (cmd/nuvem). Contexto de build: a raiz do repositório.
#   docker build -f deploy/nuvem/Dockerfile -t applyer-nuvem .
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/nuvem ./cmd/nuvem
COPY internal/nuvem ./internal/nuvem
COPY internal/licenca ./internal/licenca
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /nuvem ./cmd/nuvem

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /nuvem /nuvem
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/nuvem"]
