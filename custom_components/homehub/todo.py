"""To-do list platform mapping HomeHub shopping stores to HA Todo lists."""

from __future__ import annotations

import re

from homeassistant.components.todo import (
    TodoItem,
    TodoItemStatus,
    TodoListEntity,
    TodoListEntityFeature,
)
from homeassistant.config_entries import ConfigEntry
from homeassistant.core import HomeAssistant
from homeassistant.helpers.entity_platform import AddEntitiesCallback

from .const import DOMAIN
from .coordinator import HomeHubDataUpdateCoordinator
from .entity import HomeHubEntity


def parse_quantity_from_description(description: str | None) -> int:
    """Parse an integer quantity from an optional TodoItem description."""
    if not description:
        return 1
    match = re.search(r"(\d+)", description)
    if match:
        qty = int(match.group(1))
        if qty > 0:
            return qty
    return 1


async def async_setup_entry(
    hass: HomeAssistant,
    entry: ConfigEntry,
    async_add_entities: AddEntitiesCallback,
) -> None:
    coordinator: HomeHubDataUpdateCoordinator = hass.data[DOMAIN][
        entry.entry_id
    ]
    stores = (coordinator.data or {}).get("stores", [])
    entities = [
        HomeHubShoppingTodoListEntity(
            coordinator,
            entry.entry_id,
            int(store["id"]),
            str(store["name"]),
        )
        for store in stores
    ]
    async_add_entities(entities)


class HomeHubShoppingTodoListEntity(HomeHubEntity, TodoListEntity):
    """HomeHub shopping list exposed as a Home Assistant To-do list."""

    _attr_supported_features = (
        TodoListEntityFeature.CREATE_TODO_ITEM
        | TodoListEntityFeature.DELETE_TODO_ITEM
        | TodoListEntityFeature.UPDATE_TODO_ITEM
        | TodoListEntityFeature.SET_DESCRIPTION_ON_ITEM
    )

    def __init__(
        self,
        coordinator: HomeHubDataUpdateCoordinator,
        entry_id: str,
        store_id: int,
        store_name: str,
    ) -> None:
        super().__init__(coordinator, entry_id, f"shopping_store_{store_id}")
        self._store_id = store_id
        self._attr_name = f"Shopping: {store_name}"

    @property
    def todo_items(self) -> list[TodoItem]:
        stores = (self.coordinator.data or {}).get("stores", [])
        for store in stores:
            if int(store.get("id", 0)) == self._store_id:
                items: list[TodoItem] = []
                for raw in store.get("items", []):
                    qty = int(raw.get("quantity", 1))
                    status = (
                        TodoItemStatus.COMPLETED
                        if raw.get("checked")
                        else TodoItemStatus.NEEDS_ACTION
                    )
                    items.append(
                        TodoItem(
                            uid=str(raw["id"]),
                            summary=str(raw["name"]),
                            status=status,
                            description=f"Qty: {qty}" if qty > 1 else None,
                        )
                    )
                return items
        return []

    async def async_create_todo_item(self, item: TodoItem) -> None:
        qty = parse_quantity_from_description(item.description)
        await self.coordinator.client.async_add_shopping_item(
            store_id=self._store_id,
            name=item.summary or "",
            quantity=qty,
            checked=item.status == TodoItemStatus.COMPLETED,
        )
        await self.coordinator.async_request_refresh()

    async def async_update_todo_item(self, item: TodoItem) -> None:
        if not item.uid:
            return
        checked: bool | None = None
        if item.status is not None:
            checked = item.status == TodoItemStatus.COMPLETED
        qty: int | None = None
        if item.description is not None:
            qty = parse_quantity_from_description(item.description)
        await self.coordinator.client.async_update_shopping_item(
            int(item.uid),
            name=item.summary,
            quantity=qty,
            checked=checked,
        )
        await self.coordinator.async_request_refresh()

    async def async_delete_todo_items(self, uids: list[str]) -> None:
        for uid in uids:
            await self.coordinator.client.async_delete_shopping_item(int(uid))
        await self.coordinator.async_request_refresh()
