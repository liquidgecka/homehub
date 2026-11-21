"""Switch platform for HomeHub."""

from __future__ import annotations

from typing import Any

from homeassistant.components.switch import SwitchEntity
from homeassistant.config_entries import ConfigEntry
from homeassistant.core import HomeAssistant
from homeassistant.helpers.entity_platform import AddEntitiesCallback

from .const import DOMAIN
from .coordinator import HomeHubDataUpdateCoordinator
from .entity import HomeHubEntity


async def async_setup_entry(
    hass: HomeAssistant,
    entry: ConfigEntry,
    async_add_entities: AddEntitiesCallback,
) -> None:
    coordinator: HomeHubDataUpdateCoordinator = hass.data[DOMAIN][
        entry.entry_id
    ]
    async_add_entities([HomeHubAwaySwitch(coordinator, entry.entry_id)])


class HomeHubAwaySwitch(HomeHubEntity, SwitchEntity):
    """Switch controlling HomeHub persistent Away Mode."""

    _attr_name = "Away Mode"
    _attr_icon = "mdi:home-export-outline"

    def __init__(
        self, coordinator: HomeHubDataUpdateCoordinator, entry_id: str
    ) -> None:
        super().__init__(coordinator, entry_id, "away_mode")

    @property
    def is_on(self) -> bool | None:
        status = (self.coordinator.data or {}).get("status")
        if status is None:
            return None
        return bool(status.get("away", False))

    async def async_turn_on(self, **kwargs: Any) -> None:
        await self.coordinator.client.async_set_away(True)
        await self.coordinator.async_request_refresh()

    async def async_turn_off(self, **kwargs: Any) -> None:
        await self.coordinator.client.async_set_away(False)
        await self.coordinator.async_request_refresh()
