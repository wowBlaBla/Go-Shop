# Quick start

1. Run binary first time to create required folder structure

You will see similar to following sturcture

```
\
 \ admin
   (download latest release from https://github.com/gocommerce/goshop-admin-ui/releases/)
 \ database
   \ sqlite
     database.sqlite
 \ hugo
  \ themes
   \ default
     (download latest release from git@github.com:gocommerce/goshop-ui.git release/staging)
 + ssl
 config.toml
```

**admin** is ui 

**database** is default database folder (sqlite) 

**hugo** is static content generator folder

**ssl** is cert folder




# Development

Linter

```bash
$:~ docker run --rm -v $(pwd):/app -w /app golangci/golangci-lint:v1.35.2 golangci-lint run -v
```

Source: https://github.com/golangci/golangci-lint

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


2) Install hugo - download binary from https://github.com/gohugoio/hugo/releases
Put in /opt/hugo
```bash
$:~ ln -s /opt/hugo/hugo /bin/hugo
``` 

3) Install wrangler - https://www.npmjs.com/package/@cloudflare/wrangler 
use npm (node v12) make auth from console by

```bash
$:~ wrangler login
```

