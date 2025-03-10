"""
Database connection utilities
"""
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker

from browserfleet.server.core.settings import (
    POSTGRES_USER,
    POSTGRES_PASSWORD,
    POSTGRES_HOST,
    POSTGRES_PORT,
    POSTGRES_DB,
    DATABASE_URL,
)

class DatabaseSettings:
    """Database connection settings"""
    POSTGRES_USER: str = POSTGRES_USER
    POSTGRES_PASSWORD: str = POSTGRES_PASSWORD
    POSTGRES_HOST: str = POSTGRES_HOST
    POSTGRES_PORT: int = POSTGRES_PORT
    POSTGRES_DB: str = POSTGRES_DB
    
    @property
    def database_url(self) -> str:
        return DATABASE_URL

settings = DatabaseSettings()

engine = create_engine(settings.database_url)
SessionLocal = sessionmaker(autocommit=False, autoflush=False, bind=engine)

def get_db():
    """Get database session"""
    db = SessionLocal()
    try:
        yield db
    finally:
        db.close() 