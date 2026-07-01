# Multi-stage build for the command (write) service. Produces a static,
# non-root, distroless image (plan 03 S17b). Locales are shipped for the i18n
# error bundle (config.LocalesDir reads RECKONNA_LOCALES_DIR).
FROM docker.io/library/golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/command ./cmd/command

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/command /command
COPY --from=build /src/locales /locales
ENV RECKONNA_LOCALES_DIR=/locales
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/command"]
