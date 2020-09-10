#!/bin/bash
echo "mysql sleep query..."
mysql -u kimo -p123 -h tcpproxy -e "SELECT SLEEP(100000)" &

echo "running kimo server..."
/go/src/kimo/kimo server
