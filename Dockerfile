FROM golang:latest as build

WORKDIR /go/src/app
COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o amongusdiscord main.go


FROM alpine

VOLUME [ "/config" ]
WORKDIR /config
COPY --from=build /go/src/app/amongusdiscord /amongusdiscord

EXPOSE 8123
CMD ["/amongusdiscord"]
