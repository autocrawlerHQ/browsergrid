"""
Database connection utilities
"""
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
# todo: migrations setup
class DatabaseSettings:
    """Database connection settings"""
    POSTGRES_USER: str = "browserfleet"
    POSTGRES_PASSWORD: str = "password"  # In production, use environment variables
    POSTGRES_HOST: str = "localhost"
    POSTGRES_PORT: int = 5432
    POSTGRES_DB: str = "browserfleet"
    
    @property
    def database_url(self) -> str:
        return f"postgresql://{self.POSTGRES_USER}:{self.POSTGRES_PASSWORD}@{self.POSTGRES_HOST}:{self.POSTGRES_PORT}/{self.POSTGRES_DB}"

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