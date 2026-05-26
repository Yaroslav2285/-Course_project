# Placeholder — будет реализован на Этапе 5
FROM alpine:3.19
RUN apk add --no-cache curl
HEALTHCHECK --interval=10s --timeout=5s --retries=5 \
  CMD curl -f http://localhost:8082/health || exit 1
CMD ["sleep", "infinity"]
EXPOSE 8082
