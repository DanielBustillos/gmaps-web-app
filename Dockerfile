# Etapa 1: Compilar el ejecutable usando una imagen oficial de Go
FROM golang:1.25 AS builder
WORKDIR /app
COPY . .
ENV CGO_ENABLED=0
RUN go build -o web_server web_server.go

# Etapa 2: Imagen de ejecución basada en CentOS
FROM centos:7
WORKDIR /app
RUN yum update -y && \
    yum install -y wget && \
    yum clean all

COPY --from=builder /app/web_server .
COPY --from=builder /app/pipeline.sh .
COPY --from=builder /app/phone_scraper .
RUN chmod +x web_server pipeline.sh phone_scraper
EXPOSE 8080
CMD ["./web_server"]