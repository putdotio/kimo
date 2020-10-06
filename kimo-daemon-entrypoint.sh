#!/bin/bash
echo "mysql sleep query..."
mysql -u kimo -p123 -h tcpproxy -e "SELECT SLEEP(100000)" &

echo "running kimo daemon..."
/go/src/kimo/kimo --debug  daemon
