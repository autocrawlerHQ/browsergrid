FROM browsergrid/base:latest

LABEL org.opencontainers.image.description="Chromium browser for Browsergrid"
LABEL org.opencontainers.image.version="${BROWSER_VERSION}"
LABEL browsergrid.browser.version="${BROWSER_VERSION}"
LABEL browsergrid.playwright.version="${PLAYWRIGHT_VERSION}"

ARG PLAYWRIGHT_VERSION=latest
ARG BROWSER_VERSION=latest

RUN npm i playwright@${PLAYWRIGHT_VERSION}  


RUN npx playwright install --with-deps chromium



COPY docker/browsers/chromium/start.sh /start.sh
COPY docker/browsers/chromium/docker-healthcheck /usr/local/bin/docker-healthcheck
RUN chmod +x /start.sh /usr/local/bin/docker-healthcheck

HEALTHCHECK --interval=5s --timeout=3s --retries=3 CMD ["docker-healthcheck"]

CMD ["/start.sh"]
