#!/bin/sh

# Create fake display for RStudio
Xvfb :0 -ac -screen 0 1960x2000x24 &

# Start RStudio
/usr/bin/rserver
