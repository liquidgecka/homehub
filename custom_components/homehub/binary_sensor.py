"""Binary sensor platform for HomeHub."""

from __future__ import annotations

from homeassistant.components.binary_sensor import (
    BinarySensorDeviceClass,
    BinarySensorEntity,
)
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
    async_add_entities(
        [HomeHubDisplayBinarySensor(coordinator, entry.entry_id)]
    )


class HomeHubDisplayBinarySensor(HomeHubEntity, BinarySensorEntity):
    """Binary sensor reporting whether the HomeHub display is powered on."""

    _attr_name = "Display"
    _attr_device_class = BinarySensorDeviceClass.POWER

    def __init__(
        self, coordinator: HomeHubDataUpdateCoordinator, entry_id: str
    ) -> None:
        super().__init__(coordinator, entry_id, "display_on")

    @property
    def is_on(self) -> bool | None:
        status = (self.coordinator.data or {}).get("status")
        if status is None:
            return None
        return bool(status.get("display_on", False))
