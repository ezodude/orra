from typing import Type, Any, TypedDict

from pydantic import BaseModel


def create_typed_dict(name: str, fields: dict[str, Any]) -> Any:
    """
        Create a TypedDict from a dictionary of fields
    """
    return TypedDict(name, fields)


def create_response_model(typed_dict: Type[Any]) -> Type[BaseModel]:
    """
        Create a Pydantic model from a TypedDict
    """
    class Model(BaseModel):
        __annotations__ = typed_dict.__annotations__

    return Model
