FROM golang:1.24.5-alpine AS build
WORKDIR /src
RUN apk add --no-cache build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /user-svc ./cmd/user-svc

FROM gcr.io/distroless/static:nonroot
USER nonroot:nonroot
COPY --from=build /user-svc /user-svc
COPY --from=build /src/migrations /migrations
ENV USER_SVC_MIGRATIONS_DIR=/migrations
EXPOSE 8001
ENTRYPOINT ["/user-svc"]
