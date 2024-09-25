FROM alpine:latest
ARG SOME_PATH
RUN mkdir -p /deploy/templates /deploy/static
RUN ls /deploy
RUN ls $SOME_PATH
ADD /harness/sampleapp-linux-x64 /deploy/sampleapp-linux-x64
ADD /harness/templates /deploy/templates
ADD /harness/static /deploy/static
WORKDIR "/harness"
CMD ["./sampleapp-linux-x64"]
