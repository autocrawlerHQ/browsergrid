"""
Integration tests for FastAPI with async SQLAlchemy
"""
import pytest
from fastapi import FastAPI, Depends
from fastapi.testclient import TestClient
import asyncio
from sqlalchemy import Column, String, select
from sqlalchemy.ext.asyncio import AsyncSession, AsyncEngine, create_async_engine, async_sessionmaker
from uuid import uuid4
import contextlib
from typing import AsyncIterator

from browsergrid.server.core.db.base import Base


# Define a test model
class TestItem(Base):
    __tablename__ = "test_items"
    
    id = Column(String, primary_key=True)
    name = Column(String, nullable=False)
    description = Column(String)
    
    # Async CRUD methods for test
    @classmethod
    async def create(cls, db: AsyncSession, **kwargs):
        """Create a new test item"""
        if 'id' not in kwargs:
            kwargs['id'] = uuid4().hex
            
        item = cls(**kwargs)
        db.add(item)
        await db.commit()
        await db.refresh(item)
        return item
    
    @classmethod
    async def get(cls, db: AsyncSession, id: str):
        """Get item by ID"""
        return await db.get(cls, id)
    
    @classmethod
    async def get_all(cls, db: AsyncSession):
        """Get all items"""
        result = await db.execute(select(cls))
        return result.scalars().all()


# Create a test database session manager for integration testing
class TestDatabaseSessionManager:
    """Simplified session manager for testing"""
    
    def __init__(self):
        self._engine: AsyncEngine | None = None
        self._sessionmaker = None
    
    def init(self, url: str):
        self._engine = create_async_engine(url)
        self._sessionmaker = async_sessionmaker(bind=self._engine)
    
    async def close(self):
        if self._engine:
            await self._engine.dispose()
            self._engine = None
            self._sessionmaker = None
    
    @contextlib.asynccontextmanager
    async def connect(self) -> AsyncIterator:
        if not self._engine:
            raise Exception("Engine not initialized")
        
        async with self._engine.begin() as conn:
            yield conn
    
    @contextlib.asynccontextmanager
    async def session(self) -> AsyncIterator[AsyncSession]:
        if not self._sessionmaker:
            raise Exception("Session maker not initialized")
        
        session = self._sessionmaker()
        try:
            yield session
        except Exception:
            await session.rollback()
            raise
        finally:
            await session.close()
    
    async def create_all(self):
        if not self._engine:
            raise Exception("Engine not initialized")
        
        async with self._engine.begin() as conn:
            await conn.run_sync(Base.metadata.create_all)
    
    async def drop_all(self):
        if not self._engine:
            raise Exception("Engine not initialized")
        
        async with self._engine.begin() as conn:
            await conn.run_sync(Base.metadata.drop_all)


# Test database instance that will be reused across tests
test_db_instance = TestDatabaseSessionManager()


# Create test fixtures
@pytest.fixture(scope="module")
def event_loop():
    """Create an event loop for the test module"""
    loop = asyncio.new_event_loop()
    yield loop
    loop.close()


@pytest.fixture(scope="module")
async def test_db():
    """Set up and tear down a test database"""
    # Initialize with SQLite
    test_db_instance.init("sqlite+aiosqlite:///:memory:")
    
    # Create tables
    await test_db_instance.create_all()
    
    # Yield the manager - THIS IS KEY, yield the manager not an async generator
    yield test_db_instance
    
    # Clean up
    await test_db_instance.drop_all()
    await test_db_instance.close()


@pytest.fixture
def test_app(test_db):
    """Create a test FastAPI app with database dependency"""
    app = FastAPI()
    
    # Define dependency - Note we're using test_db directly, not as async generator
    async def get_test_db():
        async with test_db.session() as session:
            yield session
    
    # Define test routes
    @app.post("/items/", response_model=dict)
    async def create_item(name: str, description: str, db: AsyncSession = Depends(get_test_db)):
        item = await TestItem.create(db, name=name, description=description)
        return {"id": item.id, "name": item.name, "description": item.description}
    
    @app.get("/items/{item_id}", response_model=dict)
    async def get_item(item_id: str, db: AsyncSession = Depends(get_test_db)):
        item = await TestItem.get(db, item_id)
        if not item:
            return {"error": "Item not found"}
        return {"id": item.id, "name": item.name, "description": item.description}
    
    @app.get("/items/", response_model=list)
    async def get_items(db: AsyncSession = Depends(get_test_db)):
        items = await TestItem.get_all(db)
        return [{"id": item.id, "name": item.name, "description": item.description} for item in items]
    
    return app


@pytest.fixture
def client(test_app):
    """Create a test client for the FastAPI app"""
    return TestClient(test_app)


# Test Cases
def test_create_and_get_item(client):
    """Test creating and retrieving an item"""
    # Create an item
    response = client.post("/items/?name=Test%20Item&description=This%20is%20a%20test")
    assert response.status_code == 200
    
    # Check the response
    data = response.json()
    assert "id" in data
    assert data["name"] == "Test Item"
    assert data["description"] == "This is a test"
    
    # Get the item by ID
    item_id = data["id"]
    response = client.get(f"/items/{item_id}")
    assert response.status_code == 200
    
    # Verify data matches
    item = response.json()
    assert item["id"] == item_id
    assert item["name"] == "Test Item"
    assert item["description"] == "This is a test"


def test_get_all_items(client):
    """Test getting multiple items"""
    # Create a few items
    client.post("/items/?name=Item%201&description=First%20item")
    client.post("/items/?name=Item%202&description=Second%20item")
    
    # Get all items
    response = client.get("/items/")
    assert response.status_code == 200
    
    # Verify the response
    items = response.json()
    assert isinstance(items, list)
    assert len(items) >= 2
    
    # Verify item structure
    for item in items:
        assert "id" in item
        assert "name" in item
        assert "description" in item