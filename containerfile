FROM docker.io/library/golang:1.23 as builder
WORKDIR /app
COPY . .
RUN make build

FROM registry.redhat.io/ubi9-minimal@sha256:30bbd445046a3a63f5f5557a3c67dee74e3c8e7855eb0347630b020f3689823f
COPY --from=builder ./app/build/ucm /usr/bin/ucm
COPY data/ .
# RUN mkdir /opt/data && chmod -R 777 /opt/data

EXPOSE 5672

LABEL io.k8s.display-name="Starlight Application" \
      io.k8s.description="Application interface to starlight." \
      maintainer="kromashk@redhat.com"

ENTRYPOINT ["./uid_entrypoint.sh"]
