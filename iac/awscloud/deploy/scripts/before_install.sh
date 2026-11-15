#!/bin/bash
set -euo pipefail

mkdir -p /opt/werewolf/bin /etc/werewolf /var/log/app

# /etc/werewolf/ssm-path is written by cfn-init (WerewolfResourceTemplate)
ssm_path=$(cat /etc/werewolf/ssm-path)
token=$(curl -sf -X PUT http://169.254.169.254/latest/api/token -H 'X-aws-ec2-metadata-token-ttl-seconds: 60')
region=$(curl -sf -H "X-aws-ec2-metadata-token: $token" http://169.254.169.254/latest/meta-data/placement/region)

umask 077
aws ssm get-parameters-by-path --region "$region" --path "$ssm_path" --with-decryption \
  --query 'Parameters[].[Name,Value]' --output text \
  | awk -F'\t' '{n=$1; sub(".*/", "", n); print n "=" $2}' > /etc/werewolf/werewolf.env.new

[ -s /etc/werewolf/werewolf.env.new ] || { echo "no parameters under $ssm_path" >&2; exit 1; }
mv /etc/werewolf/werewolf.env.new /etc/werewolf/werewolf.env
