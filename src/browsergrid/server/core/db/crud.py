"""
Base CRUD functionality for SQLAlchemy models with async support
"""
from typing import Any, Dict, Generic, List, Optional, Type, TypeVar, Union, ClassVar
from uuid import UUID

from sqlalchemy import select, update, delete, func
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.sql.expression import Select

from browsergrid.server.core.db.base import Base

ModelType = TypeVar("ModelType")

class CRUDMixin(Generic[ModelType]):
    """
    Mixin class that adds async CRUD operations to SQLAlchemy models.
    
    Usage:
        class User(Base, CRUDMixin[User]):
            __tablename__ = "users"
            id = Column(UUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
            name = Column(String, nullable=False)
            # ...
    """
    
    @classmethod
    async def create(cls, db: AsyncSession, **kwargs) -> ModelType:
        """Create a new record"""
        obj = cls(**kwargs)
        db.add(obj)
        await db.commit()
        await db.refresh(obj)
        return obj
    
    @classmethod
    async def get(cls, db: AsyncSession, id: Any) -> Optional[ModelType]:
        """Get a record by primary key"""
        return await db.get(cls, id)
    
    @classmethod
    async def get_all(cls, db: AsyncSession) -> List[ModelType]:
        """Get all records"""
        stmt = select(cls)
        result = await db.execute(stmt)
        return result.scalars().all()
    
    @classmethod
    async def filter_by(cls, db: AsyncSession, **kwargs) -> List[ModelType]:
        """Filter records by attributes"""
        stmt = select(cls)
        for key, value in kwargs.items():
            if hasattr(cls, key):
                stmt = stmt.filter(getattr(cls, key) == value)
        result = await db.execute(stmt)
        return result.scalars().all()
    
    @classmethod
    async def get_by(cls, db: AsyncSession, **kwargs) -> Optional[ModelType]:
        """Get a single record by attributes"""
        stmt = select(cls)
        for key, value in kwargs.items():
            if hasattr(cls, key):
                stmt = stmt.filter(getattr(cls, key) == value)
        result = await db.execute(stmt)
        return result.scalars().first()
    
    @classmethod
    async def count(cls, db: AsyncSession) -> int:
        """Count total records"""
        stmt = select(func.count()).select_from(cls)
        result = await db.execute(stmt)
        return result.scalar()
    
    async def update(self, db: AsyncSession, **kwargs) -> ModelType:
        """Update record with attribute values"""
        for key, value in kwargs.items():
            if hasattr(self, key):
                setattr(self, key, value)
        await db.commit()
        await db.refresh(self)
        return self
    
    async def delete(self, db: AsyncSession) -> None:
        """Delete the record"""
        await db.delete(self)
        await db.commit()

    @classmethod
    async def bulk_create(cls, db: AsyncSession, objects_data: List[Dict[str, Any]]) -> List[ModelType]:
        """Create multiple records at once"""
        objects = [cls(**data) for data in objects_data]
        db.add_all(objects)
        await db.commit()
        
        # Refresh all objects
        for obj in objects:
            await db.refresh(obj)
            
        return objects

    @classmethod
    async def update_by_id(cls, db: AsyncSession, id: Any, **kwargs) -> Optional[ModelType]:
        """Update a record by ID without fetching it first"""
        stmt = (
            update(cls)
            .where(cls.id == id)
            .values(**kwargs)
            .returning(cls)
        )
        result = await db.execute(stmt)
        await db.commit()
        return result.scalars().first()