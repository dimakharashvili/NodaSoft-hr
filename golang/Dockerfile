FROM golang:latest
COPY . /app
WORKDIR /app

RUN go env -w CGO_ENABLED=1
RUN go env -w GOOS=linux
RUN go env -w GOARCH=amd64

CMD ["go", "run", "-race", "main.go"]