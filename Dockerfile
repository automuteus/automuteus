# TODO Make this a proper multi-stage build; build image w/ dependencies v. runtime image
FROM golang:1.14

WORKDIR /go/src/app
COPY . .

RUN go build -o amongusdiscord main.go

EXPOSE 8123

CMD ./amongusdiscord
