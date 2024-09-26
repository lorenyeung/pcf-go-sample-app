FROM alpine
RUN mkdir -p /deploy/templates /deploy/static
RUN mkdir -p /deploy/test
RUN apk add curl
COPY ./sampleapp-linux-x64 /deploy/sampleapp-linux-x64
COPY ./templates /deploy/templates
COPY ./static /deploy/static
WORKDIR "/deploy"
CMD ["./sampleapp-linux-x64"]
