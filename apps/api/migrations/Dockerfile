FROM golang:1.24-alpine AS build

RUN go install -tags='no_clickhouse no_libsql no_mssql no_mysql no_sqlite3 no_vertica no_ydb' github.com/pressly/goose/v3/cmd/goose@v3.25.0

FROM alpine:3.22

COPY --from=build /go/bin/goose /usr/local/bin/goose
COPY . /migrations

ENTRYPOINT ["sh", "-c", "goose -dir /migrations postgres \"$DATABASE_URL\" up"]
