FROM browsergrid/base:latest


RUN npm i playwright
RUN npx playwright install --with-deps chrome


COPY docker/browsers/chrome/start.sh /start.sh
RUN chmod +x /start.sh

CMD ["/start.sh"]
