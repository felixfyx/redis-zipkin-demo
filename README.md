# redis-zipkin-demo
An attempt at getting redis and zipkin working together in a docker container
## Requirements
- Docker
- Golang 1.20
## How to use
From the main folder, you may be getting errors from `redis_subscriber` and `redis_publisher` folders. To fix that run the following:
```
go work init
go work use ./redis_subscriber/src
go work use ./redis_publisher/src
```
Run the following command
```
docker compose up
```
You should be able to get the `publisher` and `redis` running.

Now go to `redis_subscriber` and run the following
```
go run main.go
```
#### Troubleshooting
If `redis_subscriber` is complaining about connection loss, you may need to modify the `config.yml` and change the host and port number
## Clean up
To clean up everything, simply stop running the `subscriber` and `docker container` and run the following:
```
docker compose down -v
```

### Notes:
#### Running on MacOS
MacOS cannot connect to docker containers via its internal IP.

At the moment, to get around this issue we have to connect to `127.0.0.1:8080` instead of the internal IP address.

Possible solution to look into:
```
brew install chipmk/tap/docker-mac-net-connect
```