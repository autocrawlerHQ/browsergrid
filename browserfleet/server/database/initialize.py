"""
Initialize database tables
"""
from browserfleet.server.models.base import Base
from browserfleet.server.database.db import engine
import browserfleet.server.models  # noqa: F401

def init_db():
    """Create all database tables"""
    Base.metadata.create_all(bind=engine)
    print("Database tables created successfully.")

if __name__ == "__main__":
    init_db()