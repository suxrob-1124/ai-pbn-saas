FROM golang:1.24 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Для WebP-конвертации в воркере нужен CGO + компилятор C
RUN apt-get update && \
    apt-get install -y --no-install-recommends build-essential pkg-config && \
    rm -rf /var/lib/apt/lists/*

COPY . .
RUN CGO_ENABLED=0 go build -o /bin/authserver ./cmd/authserver && \
    CGO_ENABLED=0 go build -o /bin/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -o /bin/healthcheck ./cmd/healthcheck && \
    CGO_ENABLED=0 go build -o /bin/scheduler ./cmd/scheduler && \
    CGO_ENABLED=0 go build -o /bin/indexchecker ./cmd/indexchecker && \
    CGO_ENABLED=1 go build -o /bin/worker ./cmd/worker

# Runtime образ: используем distroless/cc вместо base-debian12
# Причина: worker компилируется с CGO_ENABLED=1 для поддержки WebP конвертации
# distroless/cc содержит libc и другие необходимые библиотеки для CGO
# Если worker будет работать без ошибок "library not found", можно вернуться на base-debian12
FROM gcr.io/distroless/cc-debian12
WORKDIR /app
COPY --from=build /bin/authserver /bin/authserver
COPY --from=build /bin/migrate /bin/migrate
COPY --from=build /bin/healthcheck /bin/healthcheck
COPY --from=build /bin/scheduler /bin/scheduler
COPY --from=build /bin/indexchecker /bin/indexchecker
COPY --from=build /bin/worker /bin/worker
USER nonroot:nonroot
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s --retries=3 CMD ["/bin/healthcheck"]
ENTRYPOINT ["/bin/authserver"]
