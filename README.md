# OpenSessionReplay

Lightweight application to record and view sessions of your website users.

## Showcase

![](./showcase.mp4)

## Features

* Record sessions of your users using your website
* Lightweight Go backend
* Easy setup via Docker
* Admin interface to view your recorded sessions
* Works on any modern browser
* No cookies (GDPR compliant)

## Configuration / Environment variables

* PORT
  * Default 8080
* BASIC_AUTH_USER
  * Username for admin backend
  * Default "admin"
* BASIC_AUTH_PASS
  * Password for admin backend
  * Default "admin"
* PROXY_URL
  * Reverse proxy host
  * Default "admin"
* RRWEB_JS_NAME
  * Javascript file name for rrweb
  * Default "rrweb.min.js"
* RECORDER_JS_NAME
  * Javascript file name for recorder
  * Default "recorder.js"

## Setup

Change the environment variables in the docker-compose.yml for your needs. Then spin up the container as follows:

```
docker compose up -d
```

## Enable session recording on your website

Add the following lines to the end of your head section in your website.

```html
<script src="https://your.domain.com/rrweb.min.js"></script>
<script src="https://your.domain.com/recorder.js"></script>
```

The `recorder.js` must be AFTER the rrweb. If you changed the environment variables
`RRWEB_JS_NAME` and `RECORDER_JS_NAME`, then please change the path after the host according to it.

I recommend changing the two variables, because any adblocker will block rrweb, which is needed to record the sessions.

## Development

Dependencies:

* NPM latest version
* Go latest version

Install rrweb:
```
npm ci --only=production
```

Run the go application:
```
go run main.go
```
!!! Takes ages for the first time !!!

## License

GPL v3
