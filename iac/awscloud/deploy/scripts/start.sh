#!/bin/bash
set -e
cp /opt/werewolf/werewolf.service /etc/systemd/system/werewolf.service
systemctl daemon-reload
systemctl enable werewolf
systemctl restart werewolf
