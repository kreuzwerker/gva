#/bin/sh
apt-get update -qq &&
  apt-get install -qqy apt-transport-https &&
  curl https://repo.varnish-cache.org/ubuntu/GPG-key.txt | apt-key add - &&
  echo "deb https://repo.varnish-cache.org/ubuntu/ precise varnish-4.0" >> /etc/apt/sources.list.d/varnish-cache.list &&
  apt-get update -qq &&
  apt-get install -qqy varnish &&
  mkdir -p /var/lib/varnish &&
  chmod 777 /var/lib/varnish
