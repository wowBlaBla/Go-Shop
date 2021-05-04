# Quick start

1. Run binary first time to create required folder structure

You will see similar to following structure

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


# Dependientes #

Use docker for cross platform compilation

Being in main.go folder run one of following code

## Centos 7 compatible compilation ##

```
$:~ docker run --rm -v "$PWD":/usr/src/myapp -v /tmp/go/pkg:/go/pkg -w /usr/src/myapp golang:1.16-stretch go build -v -ldflags="-s -w"
```


1. Install hugo - download binary from https://github.com/gohugoio/hugo/releases
   Put in /opt/hugo
```bash
$:~ ln -s /opt/hugo/hugo /bin/hugo
``` 

3) Install wrangler - https://www.npmjs.com/package/@cloudflare/wrangler
   use npm (node v12) make auth from console by

```bash
$:~ wrangler login
```


# Development

Linter

```bash
$:~ golangci-lint run
```

Source: https://github.com/golangci/golangci-lint

## Swagger

Run to update swagger doc after annotations changes

```
$:~ swag init
```

Source: https://github.com/arsmn/fiber-swagger


