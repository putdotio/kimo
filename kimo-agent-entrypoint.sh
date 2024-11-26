#!/bin/bash

host="kimo-tcpproxy"
user="kimo"
password="123"

for i in {30..0}; do
        mysql -h"$host" -u"$user" -p"$password" -e "SELECT 1" > /dev/null
        if [ $? -eq 0 ]; then
            echo "MySQL connection successful."
            break
        fi

        echo 'MySQL init process in progress...'
        sleep 1
done
if [ "$i" = 0 ]; then
        echo >&2 'MySQL init process failed.'
        exit 1
fi

echo "mysql sleep queries..."
mysql -u"$user" -p"$password" -h"$host" -e "SELECT SLEEP(5000)" &
mysql -u"$user" -p"$password" -h"$host" -e "SELECT SLEEP(6000)" &
mysql -u"$user" -p"$password" -h"$host" -e "SELECT SLEEP(7000)" &

echo "running kimo agent..."
kimo --debug agent
