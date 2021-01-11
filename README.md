# Development

## Swagger

Run to update swagger doc after annotations changes

```
$:~ swag init
```

Source: https://github.com/arsmn/fiber-swagger

# Installation

1) Install nginx
```bash
$:~ yum install nginx
```

Configure in /etc/nginx/conf.d/your_domain_name.conf

```nginx
server {
	server_name  your_domain_name;
	root         /var/www/html/your_domain_name;
 
	include /etc/nginx/default.d/*.conf;

    location / {
        root /opt/goshop/hugo/public/;
    }

    location /admin {
        proxy_pass      http://localhost:18092;
    }

    location /api {
        proxy_pass      http://localhost:18092;
    }

    location /assets/ {
        root /opt/goshop/admin/;
    }

    location /storage/ {
        root /opt/goshop/;
    }
}
```

after it you can install certbot for nginx

```bash
$:~ yum install certbot python3-certbot-nginx
```

IMPORTANT: this installation break the system because of force redirect to https, WE DO NOT NEED THIS REDIRECT!
Edit file */etc/nginx/conf.d/your_domain_name.conf* to remove such redirect and put 

```nginx
    location /api {
        proxy_pass      http://localhost:18092;
    }
```

to http server part to make api work.

The result should be like this

```nginx
server {
	server_name  your_domain_name;
	root         /var/www/html/your_domain_name;
	include /etc/nginx/default.d/*.conf;
    location / {
        root /opt/goshop/hugo/public/;
    }
    location /admin {
        proxy_pass      http://localhost:18092;
    }
    location /api {
        proxy_pass      http://localhost:18092;
    }
    location /assets/ {
        root /opt/goshop/admin/;
    }
    location /storage/ {
        root /opt/goshop/;
    }
    listen 443 ssl; # managed by Certbot
    ssl_certificate /etc/letsencrypt/live/your_domain_name/fullchain.pem; # managed by Certbot
    ssl_certificate_key /etc/letsencrypt/live/your_domain_name/privkey.pem; # managed by Certbot
    include /etc/letsencrypt/options-ssl-nginx.conf; # managed by Certbot
    ssl_dhparam /etc/letsencrypt/ssl-dhparams.pem; # managed by Certbot
}
server {
    location /api {
        proxy_pass      http://localhost:18092;
    }
	listen       80;
	server_name  your_domain_name;
}
```

2) Install redis server to store cookies sessions

```bash
$:~ yum install redis
$:~ systemctl enable redis
$:~ systemctl start redis
```

3) Install hugo - download binary from https://github.com/gohugoio/hugo/releases
Put in /opt/hugo
```bash
$:~ ln -s /opt/hugo/hugo /bin/hugo
``` 

4) Install wrangler - https://www.npmjs.com/package/@cloudflare/wrangler 
use npm (node v12) make auth from console by

```bash
$:~ wrangler login
```

