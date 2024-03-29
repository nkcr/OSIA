# Ensure "osia-linux-amd64" is present. Then: 
#   docker build -t osia .
#   docker run -e INSTAGRAM_TOKEN=XXX -p 3333:3333 -v $(pwd)/data:/data osia
FROM alpine:3.14
COPY --chmod=0755 ./osia-linux-amd64 /osia
EXPOSE 3333
RUN mkdir /data
ENTRYPOINT ["/osia",  "--interval", "30m", "--dbfilepath", "/data/osia.db", \
  "--imagesfolder", "/data/images/", "--listen", "0.0.0.0:3333"]