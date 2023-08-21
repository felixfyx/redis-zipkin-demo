# This is where I decide to run different commands to build the program
FROM golang:1.20

# Copying all src folder stuff from host machine into opt...
COPY src /opt/redis_publisher

# Entry point in container will be here
WORKDIR /opt/redis_publisher

# ENTRYPOINT [ "echo", "Hello"  ]

# Build golang stuff
RUN go mod tidy
RUN go build -o publisher

CMD ["./publisher"]