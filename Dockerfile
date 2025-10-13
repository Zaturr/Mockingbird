FROM go:1.24.0

WORKDIR /app
ADD go.mod /app
ADD go.sum /app

COPY .. /app/

CMD  go build main.go

ENTRYPOINT ["/app/main"]