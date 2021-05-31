# Deploy new instance 

Centos 7

## 1. Update all

```bash
yum update
yum install epel-release
yum install mc git
```

## 2. Disable selinux

In file /etc/selinux/config set SELINUX=disabled, restart required

## 3. Install rpm

Download and install the latest rpm from https://github.com/gocommerce/goshop/releases

```bash
rpm -i goshop-*.x86_64.rpm
```

## 4. Install nginx

In /etc/nginx/nginx.conf in section 'http' add

```shell
    proxy_read_timeout 300;
    proxy_connect_timeout 300;
    proxy_send_timeout 300;
    client_max_body_size 100M;
```

```bash
yum install nginx
systemctl enable nginx
```

create file **/etc/nginx/conf.d/myshop.com.conf** contains config of **preview.myshop.com**

```
server {
    server_name  preview.myshop.com;
    root         /opt/goshop/hugo/public;
    error_page 404 /404.html;
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

    listen 443 ssl;
    ssl_certificate /opt/goshop/ssl/server.crt;
    ssl_certificate_key /opt/goshop/ssl/server.key;
}
```

```shell
service nginx restart
```

## 5. Install MySQL 8

Follow this guide to install MySQL 8 https://www.mysqltutorial.org/install-mysql-centos/

In file /etc/my.cnf add to the end

```shell
sql_mode=
```

(no value)

```shell
systemctl enable mysqld
service mysqld restart
```

## 6. Install php 7.4 (latest)

Follow this guide to install PHP 7.4 https://computingforgeeks.com/how-to-install-php-7-4-on-centos-7/

```shell
systemctl enable php-fpm
service php-fpm restart
```

## 7. Install phpmyadmin

Download fresh phpmyadmin from official website and put to /usr/share/phpmyadmin

Create nginx config **/etc/nginx/default.d/phpmyadmin.conf**

```shell
location /phpmyadmin {
        root /usr/share;

        location ~ \.php$ {
                fastcgi_pass 127.0.0.1:9000;
                include         fastcgi_params;
                fastcgi_param   SCRIPT_FILENAME    $document_root$fastcgi_script_name;
                fastcgi_param   SCRIPT_NAME        $fastcgi_script_name;
        }
}
```

```shell
service nginx restart
```

## 8 Configure goshop

Using phpmyadmin create new user and database 'shop'

in /opt/goshop/config.toml configure database credentials

```toml
[Database]
  Dialer = "mysql"
  Uri = "shop:mypassword@/shop?parseTime=true"
```

```shell
service goshop restart
```