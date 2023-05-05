-   [Re-streaming](#re-streaming)
-   [REST API](#rest-api)
    -   [System](#system)
    -   [General](#general)
    -   [User](#user)
    -   [Monitor](#monitor)
    -   [Recording](#recording)
    -   [Logs](#logs)
-   [Websockets API](#websockets-api)
    -   [Logs](#logs)

# Re-streaming

## RTSP

### Main rtsp\://127.0.0.1:2021/\<monitor-id\>

### Sub rtsp\://127.0.0.1:2021/\<monitor-id\>\_sub

##### example:

    ffplay -rtsp_transport tcp rtsp://127.0.0.1:2021/myMonitor
    ffplay -rtsp_transport tcp rtsp://127.0.0.1:2021/myMonitor_sub

Remember to expose the ports if you're using Docker.

## HLS

### Main http\://127.0.0.1:2022/hls/<monitor-id\>/stream.m3u8

### Sub http\://127.0.0.1:2022/hls/<monitor-id\>\_sub/stream.m3u8

##### example:

    ffplay http://127.0.0.1:2022/hls/myMonitor/stream.m3u8
    vlc http://127.0.0.1:2022/hls/myMonitor_sub/stream.m3u8

<br>
<br>

# REST API

All requests require basic auth, POST, PUT and DELETE requests need to have a matching CSRF-token in the `X-CSRF-TOKEN` header.

##### curl example:

    curl -k -u admin:pass -X GET https://127.0.0.1/api/users

## System

### GET /api/system/time-zone

##### Auth: user

System time zone location.

<br>

## General

### GET /api/general

##### Auth: admin

General settings.

<br>

### PUT /api/general/set

##### Auth: admin

Set general configuration.

<br>

## User

### GET /api/users

##### Auth: admin

Users.

<br>

### PUT /api/user/set

Set user data.

##### Auth: admin

example request:

```
{
	"id": "7phg3h7v3ayb5g2f",
	"username": "name",
	"isAdmin": false,
	"plainPassword": "pass"
}
```

<br>

### DELETE /api/user/delete?id=x

##### Auth: admin

Delete a user by id.

<br>

### GET /api/user/my-token

##### Auth: admin

CSRF-token of current user.

<br>

## Monitor

### GET /api/monitor/configs

##### Auth: admin

Uncensored monitor configuration.

<br>

### DELETE /api/monitor/delete?id=x

##### Auth: admin

Delete a monitor by id.

<br>

### GET /api/monitor/list

##### Auth: user

Censored monitor configuration.

<br>

### POST /api/monitor/restart?id=x

##### Auth: admin

Restart monitor by id.

<br>

### PUT /api/monitor/set

##### Auth: admin

Set monitor.

<br>

## Recording

### DELETE /api/recording/delete/\<recording-id>

##### Auth: admin

Delete recording by id.

<br>

### GET /api/recording/thumbnail/\<recording-id>

##### Auth: user

Thumbnail by exact recording ID.

<br>

### GET /api/recording/video/\<recording-id>

##### Auth: user

Video by exact recording ID.

<br>

### GET /api/recording/query?limit=1&time=2025-12-28_23-59-59&reverse=true&monitors=m1,m2&data=true

##### Auth: user

Query recordings.

example response: data=false

```
[
  {
    "id":"YYYY-MM-DD_hh-mm-ss_id",
    "data": null
  }
]
```

example response: data=true

```
[{
  "id":"YYYY-MM-DD_hh-mm-ss_id",
  "data": {
    "start": "YYYY-MM-DDThh:mm:ss.000000000Z",
    "end": "YYYY-MM-DDThh:mm:ss.000000000Z",
    "events": [{
        "time": "YYYY-MM-DDThh:mm:ss.000000000Z",
        "detections": [{
            "label": "person",
            "score": 100,
            "region": {
              "rect": [0, 0, 100, 100]
            }
        }],
        "duration": 000000000
}]}}]
```

<br>
## Logs

### GET /api/log/query?levels=16,24&sources=app,monitors=a,b&time=1234567890111222&limit=2

##### Auth: admin

Query logs. Time is in Unix micro seconds.

example response:

```
[
  {
    "level":0,
    "time":0,
    "msg":"",
    "src":"",
    "monitorID":""
  },
  {
    "level":0,
    "time":0,
    "msg":"",
    "src":"",
    "monitorID":""
  }
]
```

<br>

### GET /api/log/sources

##### Auth: admin

List of log sources.

example response:`["app","monitor","recorder","storage","watchdog"]`

<br>
<br>

# Websockets API

Requires basic auth and TLS. Authentication is validated before each response.

example: `wss://127.0.0.1/api/logs`

curl doesn't support wss.

## Logs

### /api/logs?levels=16,24&monitors=a,b&sources=app,monitor

##### Auth: admin

Live log feed.
