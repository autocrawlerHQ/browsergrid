"""
Database connection utilities with async SQLAlchemy 2.0
"""
import contextlib
from typing import AsyncIterator

from sqlalchemy.ext.asyncio import (
    AsyncConnection, 
    AsyncEngine, 
    AsyncSession,
    async_sessionmaker, 
    create_async_engine
)

from browsergrid.server.core.settings import (
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
        # Convert regular DB URL to async format
        # Example: postgresql:// → postgresql+asyncpg://
        if DATABASE_URL.startswith('postgresql://'):
            return DATABASE_URL.replace('postgresql://', 'postgresql+asyncpg://', 1)
        return DATABASE_URL

settings = DatabaseSettings()

class DatabaseSessionManager:
    def __init__(self):
        self._engine: AsyncEngine | None = None
        self._sessionmaker: async_sessionmaker | None = None

    def init(self, host: str):
        self._engine = create_async_engine(host)
        self._sessionmaker = async_sessionmaker(autocommit=False, bind=self._engine)

    async def close(self):
        if self._engine is None:
            raise Exception("DatabaseSessionManager is not initialized")
        await self._engine.dispose()
        self._engine = None
        self._sessionmaker = None

    @contextlib.asynccontextmanager
    async def connect(self) -> AsyncIterator[AsyncConnection]:
        if self._engine is None:
            raise Exception("DatabaseSessionManager is not initialized")

        async with self._engine.begin() as connection:
            try:
                yield connection
            except Exception:
                await connection.rollback()
                raise

    @contextlib.asynccontextmanager
    async def session(self) -> AsyncIterator[AsyncSession]:
        if self._sessionmaker is None:
            raise Exception("DatabaseSessionManager is not initialized")

        session = self._sessionmaker()
        try:
            yield session
        except Exception:
            await session.rollback()
            raise
        finally:
            await session.close()

    # Used for migrations and testing
    async def create_all(self, connection: AsyncConnection):
        from browsergrid.server.core.db.base import Base
        await connection.run_sync(Base.metadata.create_all)

    async def drop_all(self, connection: AsyncConnection):
        from browsergrid.server.core.db.base import Base
        await connection.run_sync(Base.metadata.drop_all)

sessionmanager = DatabaseSessionManager()

async def get_db():
    async with sessionmanager.session() as session:
        yield session