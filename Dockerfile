# Stage 1: Build the Go application
FROM public.ecr.aws/docker/library/golang:latest AS build

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a static Linux executable
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Stage 2: Create a smaller runtime image
FROM public.ecr.aws/docker/library/alpine:latest

# Install HTTPS certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy only the compiled application
COPY --from=build /build/main ./

EXPOSE 8080

CMD ["./main"]
