<div align="center">
<img width="180" src="logo.jpg"/>
</div>

# Open Source Instagram Aggregator

[![Go Tests](https://github.com/nkcr/OSIA/actions/workflows/go.yml/badge.svg)](https://github.com/nkcr/OSIA/actions/workflows/go.yml)
[![Coverage Status](https://coveralls.io/repos/github/nkcr/OSIA/badge.svg?branch=main)](https://coveralls.io/github/nkcr/OSIA?branch=main)

OSIA stores your public instagram feed on a DB and offers a simple REST API to
later use your it wherever you want. This is especially convenient to display
your instagram posts on a website.

The application is composed of an aggregator, which periodically fetches the
Instagram post and saves them on a local database, and an HTTP server, which
serves posts from the local database. 

## Requirement

You can use the existing binaries from the [releases
section](https://github.com/nkcr/OSIA/releases), which are completely
self-contained. No additional installation is needed. You can also compile the
app yourself, from the root folder:

```sh
# build a binary
go build .
# build a binary and moves it to $GOBIN
go install .
```

## Usage

The Instagram token must be passed as an environment variable, and settings can
be passed via the CLI. To see the options, use `-h`. For example:

```sh
INSTAGRAM_TOKEN=XXX go run . --interval 30m --dbfilepath data/osia.db --imagesfolder images/ --listen 0.0.0.0:3333
```

The app can be stopped with <kbd>Ctrl</kbd> + <kbd>C</kbd>. To prevent a full
download, it can be re-started with the same database and images folder.

## Read your posts

An HTTP server is bootstrapped at the provided (or default) `listen` address. It
serves the list of medias at the `http://<listen>/api/medias` endpoint. By
default, the endpoint returns a maximum of 12 medias, sorted by timestamp.
Recall that your endpoint is likely to be public, you don't want to expose too
much data.

It is possible to retrieve less than 12 medias by specifying a `count=` URL
parameter:

```
# Returns the last 8 posts
http://0.0.0.0:3333/api/medias?count=8
```

A post has the following attributes:

```
{
  id:
  caption:
  media_type:
  media_url:
  permalink:
  username:
  timestamp:
}
```

## Images

Due to Instagram security restrictions, images hosted by Instagram cannot be
displayed on external websites. Consequentely, a simple `<img src={permalink}/>`
tag would not work. To get around that, images are saved locally to the provided
(or default) `imagesfolder` and served at the `http://<listen>/images/<post
id>.jpg` endpoint. "media id" corresponds to the `id` of the post. 