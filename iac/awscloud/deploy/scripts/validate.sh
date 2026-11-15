#!/bin/bash
# The service must survive its first seconds (config errors make the binary exit at startup).
sleep 5
systemctl is-active --quiet werewolf && exit 0

# CodeDeploy keeps only the last lines of script output, so the app log goes last.
echo "werewolf is not active"
systemctl status werewolf --no-pager -l 2>&1 | tail -n 10
echo "--- /var/log/app/werewolf.log"
tail -n 15 /var/log/app/werewolf.log 2>&1
exit 1
