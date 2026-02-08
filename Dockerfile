# Build stage
FROM golang:1.22-alpine AS build
WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /ironclaw ./cmd/ironclaw

# Run stage
FROM alpine:3.19
RUN adduser -D -g "" ironclaw
WORKDIR /app

COPY --from=build /ironclaw .
COPY ironclaw.json VERSION ./
RUN chown -R ironclaw:ironclaw /app

USER ironclaw
EXPOSE 8080

ENTRYPOINT ["./ironclaw"]
