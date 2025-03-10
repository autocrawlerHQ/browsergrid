FROM python:3.12-slim

ENV PYTHONPATH=/app

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    libpq-dev \
    netcat-traditional \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

    
    
COPY requirements.txt .

RUN pip install -r requirements.txt

COPY . .

COPY docker/server/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 8765




ENTRYPOINT ["/entrypoint.sh"] 