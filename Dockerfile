FROM golang:latest AS build

WORKDIR /go/src/app

COPY . .

RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o app .

FROM centurylink/ca-certs

WORKDIR /app

COPY --from=build /go/src/app/app .

CMD ["./app"]