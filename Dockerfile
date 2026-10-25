FROM golang:1.27.1-alpine

WORKDIR /app

COPY . .

RUN go mod download

RUN go build -o server .

EXPOSE 8080

CMD ["./server"]