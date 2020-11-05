FROM golang:1.15-alpine AS builder

# Git is required for getting the dependencies.
# hadolint ignore=DL3018
RUN apk add --no-cache git

WORKDIR /src

# Fetch dependencies first; they are less susceptible to change on every build
# and will therefore be cached for speeding up the next build
COPY ./go.mod ./go.sum ./
RUN go mod download

# Import the code from the context.
COPY ./ ./

# Build the executable to `/app`. Mark the build as statically linked.
# hadolint ignore=SC2155
RUN export TAG=$(git describe --tags "$(git rev-list --tags --max-count=1)") && \
    export COMMIT=$(git rev-parse --short HEAD) && \
    CGO_ENABLED=0 \
    go build -installsuffix 'static' \
    -ldflags="-X main.version=${TAG} -X main.commit=${COMMIT}" \
    -o /app .

FROM alpine:3.12.1 AS final

# Set up non-root user and app directory
# * Non-root because of the principle of least privlege
# * App directory to allow mounting volumes
RUN addgroup -g 1000 bot && \
    adduser -HD -u 1000 -G bot bot && \
    mkdir -p /app/logs /app/config && \
    chown -R bot:bot /app
USER bot

# Import the compiled executable from the first stage.
COPY --from=builder /app /app

# Port used for capture program to report back
EXPOSE 8123
# Port used for application command and control
EXPOSE 5000

ENV CONFIG_PATH="/app/config" \
    LOG_PATH="/app/logs"
VOLUME ["/app/config", "/app/logs"]

# Run the compiled binary.
ENTRYPOINT ["/app/app"]
