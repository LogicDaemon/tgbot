#!/bin/bash

set -xe

go build -o /usr/local/bin/repost_magti_news ./repost_magti_news.go
if crontab -l | grep '\brepost_magti_news\b' -q; then
	echo "Cron job already exists, skipping addition."
	exit
fi
croncmd="0 * * * * repost_magti_news"
(crontab -l 2>/dev/null; echo "$croncmd") | crontab -
