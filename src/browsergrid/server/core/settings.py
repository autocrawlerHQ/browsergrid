import os
import uuid


# SECURITY WARNING: keep the secret key used in production secret!
# In production, this should be set through environment variables
SECRET_KEY = os.environ.get("BROWSERGRID_SECRET_KEY", 'secret')

# SECURITY WARNING: don't run with debug turned on in production!
DEBUG = os.environ.get("BROWSERGRID_DEBUG", "False").lower() in ("true", "1", "t")

INSTALLED_APPS = [
    "browsergrid.server.sessions",
    "browsergrid.server.webhooks",
    "browsergrid.server.workerpool",
]

# API configuration
API_HOST = os.environ.get("BROWSERGRID_API_HOST", "127.0.0.1")
API_PORT = int(os.environ.get("BROWSERGRID_API_PORT", "8765"))
API_KEY = os.environ.get("BROWSERGRID_API_KEY", None)
SERVER_ID = os.environ.get("BROWSERGRID_SERVER_ID", str(uuid.uuid4()))

# API Authentication configuration
API_AUTH = {
    "key": API_KEY,
    "header_name": "X-API-Key",
    "excluded_paths": [
        "/docs",
        "/redoc",
        "/openapi.json",
        "/health",
    ]
}

# UI configuration
UI_ENABLED = os.environ.get("BROWSERGRID_UI_ENABLED", "True").lower() in ("true", "1", "t")

# Database configuration
POSTGRES_USER = os.environ.get("BROWSERGRID_POSTGRES_USER", "browsergrid")
POSTGRES_PASSWORD = os.environ.get("BROWSERGRID_POSTGRES_PASSWORD", "password")
POSTGRES_HOST = os.environ.get("BROWSERGRID_POSTGRES_HOST", "localhost")
POSTGRES_PORT = int(os.environ.get("BROWSERGRID_POSTGRES_PORT", "5432"))
POSTGRES_DB = os.environ.get("BROWSERGRID_POSTGRES_DB", "browsergrid")
DATABASE_URL = f"postgresql://{POSTGRES_USER}:{POSTGRES_PASSWORD}@{POSTGRES_HOST}:{POSTGRES_PORT}/{POSTGRES_DB}" 



# MIDDLEWARE settings
CORS_ALLOW_ORIGINS = os.environ.get("BROWSERGRID_CORS_ALLOW_ORIGINS", "*").split(",")
CORS_ALLOW_CREDENTIALS = os.environ.get("BROWSERGRID_CORS_ALLOW_CREDENTIALS", "True").lower() in ("true", "1", "t")
CORS_ALLOW_METHODS = os.environ.get("BROWSERGRID_CORS_ALLOW_METHODS", "*").split(",")
CORS_ALLOW_HEADERS = os.environ.get("BROWSERGRID_CORS_ALLOW_HEADERS", "*").split(",")

# Rate limiting
RATE_LIMIT_ENABLED = os.environ.get("BROWSERGRID_RATE_LIMIT_ENABLED", "False").lower() in ("true", "1", "t")
RATE_LIMIT_REQUESTS_PER_MINUTE = int(os.environ.get("BROWSERGRID_RATE_LIMIT_REQUESTS_PER_MINUTE", "120"))

# Compression
GZIP_ENABLED = os.environ.get("BROWSERGRID_GZIP_ENABLED", "True").lower() in ("true", "1", "t")
GZIP_MINIMUM_SIZE = int(os.environ.get("BROWSERGRID_GZIP_MINIMUM_SIZE", "1000"))

MIDDLEWARE = [
    # CORS middleware should be first in the chain
    {
        "class": "fastapi.middleware.cors.CORSMiddleware",
        "kwargs": {
            "allow_origins": CORS_ALLOW_ORIGINS,
            "allow_credentials": CORS_ALLOW_CREDENTIALS,
            "allow_methods": CORS_ALLOW_METHODS,
            "allow_headers": CORS_ALLOW_HEADERS,
        },
    },
    {
        "class": "browsergrid.server.core.middlewares.ApiKeyMiddleware",
        "kwargs": {
            "api_key": API_AUTH["key"],
            "excluded_paths": API_AUTH["excluded_paths"],
        },
    },
    # GZip compression
    {
        "class": "fastapi.middlewares.gzip.GZipMiddleware",
        "kwargs": {
            "minimum_size": GZIP_MINIMUM_SIZE,
        },
    },
    # Request logging
    {
        "class": "browsergrid.server.core.middlewares.RequestLogMiddleware",
        "kwargs": {
            "log_level": "info" if DEBUG else "error"
        },
    },
    {
        "class": "browsergrid.server.core.middlewares.RateLimitMiddleware",
        "kwargs": {
            "requests_per_minute": RATE_LIMIT_REQUESTS_PER_MINUTE,
            "block_time": 60,
        },
    }
]


