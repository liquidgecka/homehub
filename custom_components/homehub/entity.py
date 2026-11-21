"""Base entity for HomeHub."""

from __future__ import annotations

from homeassistant.helpers.device_registry import DeviceInfo
from homeassistant.helpers.update_coordinator import CoordinatorEntity

from .const import DOMAIN
from .coordinator import HomeHubDataUpdateCoordinator


class HomeHubEntity(CoordinatorEntity[HomeHubDataUpdateCoordinator]):
    """Shared base entity providing HomeHub device metadata."""

    _attr_has_entity_name = True

    def __init__(
        self,
        coordinator: HomeHubDataUpdateCoordinator,
        entry_id: str,
        unique_suffix: str,
    ) -> None:
        super().__init__(coordinator)
        self._entry_id = entry_id
        self._attr_unique_id = f"{entry_id}_{unique_suffix}"
        self._attr_device_info = DeviceInfo(
            identifiers={(DOMAIN, entry_id)},
            name="HomeHub",
            manufacturer="HomeHub",
            model="Touchscreen Dashboard",
        )
