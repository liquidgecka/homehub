"""Button platform for HomeHub."""

from __future__ import annotations

from homeassistant.components.button import ButtonDeviceClass, ButtonEntity
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
        [
            HomeHubRestartButton(coordinator, entry.entry_id),
            HomeHubNextPhotoButton(coordinator, entry.entry_id),
        ]
    )


class HomeHubRestartButton(HomeHubEntity, ButtonEntity):
    """Button to restart the HomeHub application in place."""

    _attr_name = "Restart"
    _attr_device_class = ButtonDeviceClass.RESTART

    def __init__(
        self, coordinator: HomeHubDataUpdateCoordinator, entry_id: str
    ) -> None:
        super().__init__(coordinator, entry_id, "restart")

    async def async_press(self) -> None:
        await self.coordinator.client.async_restart()


class HomeHubNextPhotoButton(HomeHubEntity, ButtonEntity):
    """Button to advance the HomeHub slideshow to the next photo."""

    _attr_name = "Next Photo"
    _attr_icon = "mdi:image-move"

    def __init__(
        self, coordinator: HomeHubDataUpdateCoordinator, entry_id: str
    ) -> None:
        super().__init__(coordinator, entry_id, "next_photo")

    async def async_press(self) -> None:
        await self.coordinator.client.async_next_photo()
