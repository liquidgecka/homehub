"""HomeHub integration setup."""

from __future__ import annotations

import voluptuous as vol

from homeassistant.config_entries import ConfigEntry
from homeassistant.const import CONF_HOST, CONF_PORT
from homeassistant.core import HomeAssistant, ServiceCall
from homeassistant.helpers.aiohttp_client import async_get_clientsession

from .api import HomeHubApiClient
from .const import (
    CONF_API_TOKEN,
    CONF_SCAN_INTERVAL,
    DEFAULT_PORT,
    DEFAULT_SCAN_INTERVAL,
    DOMAIN,
    PLATFORMS,
    SERVICE_ACKNOWLEDGE_REMINDER,
    SERVICE_DISMISS_ALERT,
    SERVICE_SEND_ALERT,
)
from .coordinator import HomeHubDataUpdateCoordinator


async def async_setup_entry(hass: HomeAssistant, entry: ConfigEntry) -> bool:
    """Set up HomeHub from a config entry."""
    host = entry.data[CONF_HOST]
    port = entry.data.get(CONF_PORT, DEFAULT_PORT)
    token = entry.data.get(CONF_API_TOKEN, "")
    scan_interval = entry.options.get(CONF_SCAN_INTERVAL, DEFAULT_SCAN_INTERVAL)

    session = async_get_clientsession(hass)
    client = HomeHubApiClient(session, host, port, token)
    coordinator = HomeHubDataUpdateCoordinator(hass, client, scan_interval)
    await coordinator.async_config_entry_first_refresh()

    hass.data.setdefault(DOMAIN, {})[entry.entry_id] = coordinator
    await hass.config_entries.async_forward_entry_setups(entry, PLATFORMS)

    async def handle_send_alert(call: ServiceCall) -> None:
        for coord in hass.data.get(DOMAIN, {}).values():
            await coord.client.async_send_alert(
                message=call.data["message"],
                title=call.data.get("title", "Home Assistant"),
                level=call.data.get("level", "info"),
                duration_seconds=int(call.data.get("duration_seconds", 20)),
            )

    async def handle_dismiss_alert(call: ServiceCall) -> None:
        for coord in hass.data.get(DOMAIN, {}).values():
            await coord.client.async_dismiss_alert()

    async def handle_ack_reminder(call: ServiceCall) -> None:
        reminder_id = int(call.data["reminder_id"])
        for coord in hass.data.get(DOMAIN, {}).values():
            await coord.client.async_acknowledge_reminder(reminder_id)
            await coord.async_request_refresh()

    if not hass.services.has_service(DOMAIN, SERVICE_SEND_ALERT):
        hass.services.async_register(
            DOMAIN,
            SERVICE_SEND_ALERT,
            handle_send_alert,
            schema=vol.Schema(
                {
                    vol.Required("message"): str,
                    vol.Optional("title", default="Home Assistant"): str,
                    vol.Optional("level", default="info"): str,
                    vol.Optional("duration_seconds", default=20): int,
                }
            ),
        )
    if not hass.services.has_service(DOMAIN, SERVICE_DISMISS_ALERT):
        hass.services.async_register(
            DOMAIN, SERVICE_DISMISS_ALERT, handle_dismiss_alert
        )
    if not hass.services.has_service(DOMAIN, SERVICE_ACKNOWLEDGE_REMINDER):
        hass.services.async_register(
            DOMAIN,
            SERVICE_ACKNOWLEDGE_REMINDER,
            handle_ack_reminder,
            schema=vol.Schema({vol.Required("reminder_id"): int}),
        )

    entry.async_on_unload(entry.add_update_listener(_async_reload_entry))
    return True


async def _async_reload_entry(hass: HomeAssistant, entry: ConfigEntry) -> None:
    await hass.config_entries.async_reload(entry.entry_id)


async def async_unload_entry(hass: HomeAssistant, entry: ConfigEntry) -> bool:
    """Unload a HomeHub config entry."""
    unload_ok = await hass.config_entries.async_unload_platforms(
        entry, PLATFORMS
    )
    if unload_ok:
        hass.data.get(DOMAIN, {}).pop(entry.entry_id, None)
        if not hass.data.get(DOMAIN):
            for svc in (
                SERVICE_SEND_ALERT,
                SERVICE_DISMISS_ALERT,
                SERVICE_ACKNOWLEDGE_REMINDER,
            ):
                if hass.services.has_service(DOMAIN, svc):
                    hass.services.async_remove(DOMAIN, svc)
    return unload_ok
