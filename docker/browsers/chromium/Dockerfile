FROM browsergrid/base:latest


RUN npm i playwright
RUN npx playwright install --with-deps chromium


COPY docker/browsers/chromium/start.sh /start.sh
COPY docker/browsers/chromium/docker-healthcheck /usr/local/bin/docker-healthcheck
RUN chmod +x /start.sh /usr/local/bin/docker-healthcheck

HEALTHCHECK --interval=5s --timeout=3s --retries=3 CMD ["docker-healthcheck"]

CMD ["/start.sh"]
