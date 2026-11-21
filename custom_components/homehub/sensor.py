"""Sensor platform for HomeHub."""

from __future__ import annotations

from typing import Any

from homeassistant.components.sensor import SensorEntity
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
            HomeHubPendingRemindersSensor(coordinator, entry.entry_id),
            HomeHubActiveViewSensor(coordinator, entry.entry_id),
        ]
    )


class HomeHubPendingRemindersSensor(HomeHubEntity, SensorEntity):
    """Sensor reporting unacknowledged HomeHub reminders."""

    _attr_name = "Pending Reminders"
    _attr_icon = "mdi:bell-ring-outline"
    _attr_native_unit_of_measurement = "reminders"

    def __init__(
        self, coordinator: HomeHubDataUpdateCoordinator, entry_id: str
    ) -> None:
        super().__init__(coordinator, entry_id, "pending_reminders")

    @property
    def native_value(self) -> int:
        reminders = (self.coordinator.data or {}).get("reminders", {})
        return int(reminders.get("pending_count", 0))

    @property
    def extra_state_attributes(self) -> dict[str, Any]:
        reminders = (self.coordinator.data or {}).get("reminders", {})
        pending = [
            {"id": r.get("id"), "title": r.get("title"), "time": r.get("time")}
            for r in reminders.get("reminders", [])
            if r.get("enabled")
            and not r.get("acknowledged")
            and r.get("last_triggered")
        ]
        return {"pending": pending}


class HomeHubActiveViewSensor(HomeHubEntity, SensorEntity):
    """Sensor reporting the currently selected HomeHub touchscreen view."""

    _attr_name = "Active View"
    _attr_icon = "mdi:monitor-dashboard"

    def __init__(
        self, coordinator: HomeHubDataUpdateCoordinator, entry_id: str
    ) -> None:
        super().__init__(coordinator, entry_id, "active_view")

    @property
    def native_value(self) -> str | None:
        status = (self.coordinator.data or {}).get("status", {})
        return status.get("active_view")
