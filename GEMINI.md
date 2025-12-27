This directory contains a desktop application written in golang that is
intended to run on a touchscreen enabled Ubuntu machine. It allows a family to
coordinate activities and events as well as track status.

The basic approach is to have buttons on the left of the screen that are simple
icons. Each represents a view into a component of the application. When that
view is selected its icon will have a blue background to highlight that it is
active and when that view is not selected it will have a black background.

Each view is outlined here.

HOME
----

The home loads pictures from google photos, scrolling through them at rate that
can be configured on config.toml. In the bottom right of this view, overlaid
over the photo space is a heart. If touched this image will be considered a
favorite and will be displayed mo often in the feed. When showing an image that
is a favorite the heart icon should already be read. If its pressed while its
red then the image will be unfavorited and returned to normal status. To the
bottom left is a thumbs down and if selected the image will not be displayed
anymore.

PHOTOS MANAGER
--------------

This is a view that allows the user to see the photos currently configured,
heart them or remove them manually.

EVENTS
------

The Events view is a monthly calendar that displays all events that are on the
google calendars that are configured in config.toml. This view should look like
a wall hung calendar, with a grid view showing each day of the month with
events from that day displayed inside of them. Each event should be sorted by
its start time as well.

SHOPPING LIST
-------------

This is a view that allows managing shopping lists for various
stores. The stores should be displayed at the top of the view using an image
that is configurable and read from disk. This specifically should support svg.
When a store is selected the shopping list should be rendered as a list with
checkboxes allowing users to mark if they have purchased an item. Items can be
added via a big plus button that displays a form asking for the new items
name and quantity.

WEATHER
-------

The weather view should use openweathermap's 3.0 api to get weather information
and display it like a new station. The top should be a large section showing
the current weather with an icon representing conditions. Example icons include
stars for night time, clouds for cloudy, a sun for sunny, clouds with raid when
its raining, clouds with snow when its snowing, etc. This should show the
current temperature as well as the expected high and low for the day. Under
today's weather there should be 10 panels showing the coming 10 day forecast.
Each should show a smaller icon representing the expected conditions, as well
as a high and low for the day.

LEDGER
------

This view is a ledger that tracks a savings account and expenses. In this view
there should be tabs at the top with each ledger and a plus icon on the far
right to add a new ledger to the tab list. When selected a ledger will show a
running list of expenses sorted by date and with the amount rendered in green
if it added money and red if it subtracted money. At the bottom there should be
a larger number displaying the grand total for the ledger.

SECURITY
--------

The security view is a tiled view of configured RTSP streams from security
cameras. These are configured in the config file. These are not dynamically
configured via the UI. They can only be configured in the config file. The
configuration should be structured to support multiple types of video streams.
The view will then tile each camera together onto the main page. If any
individual camera is selected then it should be made to fill the screen next to
the navigation window with a button in the corner overlaid on the image that
allows the user to return to the main security view. The camera views should
only be fetched when the security view is open. No need to pre-fetch the
content.

As of now the only stream supported should be a jpeg style fetch from a frigate
server's api. This will require the URL to be configured that points to the
jpeg, as well as a refresh interval for that specific camera. This will also
require configuration of authentication credentials that can be used to fetch
the image.

WORKTREE
========

This application uses git but I do not want the agent to work with git at all.
I use git to save state when I know the app is working rather than expecting
the agent to mage any git related commits. The path to the module is
https://github.com/liquidgecka/homehub and thus the module import is
github.com/liquidgecka/homehub as per go.mod.

RUNNING THE APP
===============

Gemini shouldn't ever start the app directly as it takes over the screen. It
should direct the user to run the ap and inform them what debugging steps need
to be run and how to get the information its looking for.

CLEANLINESS
===========

All go files should be go formatted using `go fmt`.

Make sure that all standard library imports are in the first block in import,
and all third party modules imports are in the second block, and all imports
from this module are in the third block. Each block should be separated by a
single new line. All comments in specific imports can be removed as they are
not necessary.

The script 'packaging/scripts/check' can be run in order to verify that the
code is in a correct state. Please run that to verify that code is formatted,
and tests properly. This tool is always safe to run.

LICENSE
=======

All source files should be licensed under the Apache 2.0 license, copyrighted
2026 - Brady Catherman and should include a header at the top of the go file
with the standard Apache license boilerplate.
