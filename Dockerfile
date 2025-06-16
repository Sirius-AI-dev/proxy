FROM debian:stable

ENV LOG_LEVEL=info

RUN apt-get update
RUN apt-get install -y ca-certificates

ADD ./build/proxy /proxy
ADD ./config/default.yml /config.yml

ENTRYPOINT /proxy --config=/config.yml