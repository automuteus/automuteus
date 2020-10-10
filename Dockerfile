FROM golang:1.15-alpine AS builder

# Git is required for getting the dependencies.
RUN apk add --no-cache git

WORKDIR /src

# Fetch dependencies first; they are less susceptible to change on every build
# and will therefore be cached for speeding up the next build
COPY ./go.mod ./go.sum ./
RUN go mod download

# Import the code from the context.
COPY ./ ./

# Build the executable to `/app`. Mark the build as statically linked.
RUN CGO_ENABLED=0 go build \
    -installsuffix 'static' \
    -o /app .

FROM alpine AS final

# Import the compiled executable from the first stage.
COPY --from=builder /app /app

EXPOSE 8123

# Run the compiled binary.
ENTRYPOINT ["/app"]
