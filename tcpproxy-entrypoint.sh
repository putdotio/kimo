#!/bin/bash
echo "running kimo agent..."
kimo agent &

echo "running tcpproxy..."
/app/tcpproxy -m 0.0.0.0:3307 -s /var/mysql-proxy-state 0.0.0.0:3306 mysql:3306
