"""DataUpdateCoordinator for HomeHub."""

from __future__ import annotations

from datetime import timedelta
import logging
from typing import Any

from homeassistant.core import HomeAssistant
from homeassistant.exceptions import ConfigEntryAuthFailed
from homeassistant.helpers.update_coordinator import (
    DataUpdateCoordinator,
    UpdateFailed,
)

from .api import (
    HomeHubApiClient,
    HomeHubApiError,
    HomeHubAuthError,
    HomeHubConnectionError,
)
from .const import DOMAIN

_LOGGER = logging.getLogger(__name__)


class HomeHubDataUpdateCoordinator(DataUpdateCoordinator[dict[str, Any]]):
    """Coordinator that polls HomeHub /api/v1 endpoints."""

    def __init__(
        self,
        hass: HomeAssistant,
        client: HomeHubApiClient,
        scan_interval_seconds: int,
    ) -> None:
        super().__init__(
            hass,
            _LOGGER,
            name=DOMAIN,
            update_interval=timedelta(seconds=scan_interval_seconds),
        )
        self.client = client

    async def _async_update_data(self) -> dict[str, Any]:
        try:
            status = await self.client.async_get_status()
            shopping = await self.client.async_get_shopping()
            reminders = await self.client.async_get_reminders()
        except HomeHubAuthError as err:
            raise ConfigEntryAuthFailed(err) from err
        except (HomeHubConnectionError, HomeHubApiError) as err:
            raise UpdateFailed(err) from err

        return {
            "status": status,
            "stores": shopping.get("stores", []),
            "reminders": reminders,
        }
